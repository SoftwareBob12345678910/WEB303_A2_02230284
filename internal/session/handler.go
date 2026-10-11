package session

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sessionpb "stargazer/gen/session/v1"
	telescopepb "stargazer/gen/telescope/v1"
)

// telescopeAvailable mirrors the status value defined by Service B's contract.
const telescopeAvailable = "AVAILABLE"

// TelescopeLookup is what Service A needs from Service B. Depending on an
// interface keeps the handler testable and the resilience wrapper swappable.
type TelescopeLookup interface {
	GetTelescope(ctx context.Context, id string) (*telescopepb.Telescope, error)
}

// Server implements the SessionService gRPC API.
type Server struct {
	sessionpb.UnimplementedSessionServiceServer
	repo       *Repository
	telescopes TelescopeLookup
}

func NewServer(repo *Repository, telescopes TelescopeLookup) *Server {
	return &Server{repo: repo, telescopes: telescopes}
}

// CreateSession validates input, then CONFIRMS THE TELESCOPE EXISTS AND IS
// AVAILABLE by calling Service B, and only then stores the booking.
// If Service B cannot be reached we refuse (Unavailable): booking a
// telescope we cannot verify would break the business rule.
func (srv *Server) CreateSession(ctx context.Context, req *sessionpb.CreateSessionRequest) (*sessionpb.Session, error) {
	sess := Session{
		ID:          uuid.NewString(),
		TelescopeID: req.GetTelescopeId(),
		Observer:    req.GetObserver(),
		Date:        req.GetDate(),
		Target:      req.GetTarget(),
		Status:      req.GetStatus(),
	}
	sess.Normalize()
	if sess.Status == "" {
		sess.Status = StatusBooked
	}
	if sess.Status != StatusBooked {
		return nil, status.Error(codes.InvalidArgument, "new sessions must have status BOOKED")
	}
	if err := sess.Validate(true, time.Now()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	tel, err := srv.lookupBookable(ctx, sess.TelescopeID)
	if err != nil {
		return nil, err
	}
	if err := srv.repo.Create(ctx, sess); err != nil {
		return nil, repoError("create session", err)
	}

	out := toProto(sess)
	applyTelescope(out, tel)
	return out, nil
}

// GetSession returns the session enriched with the telescope's name.
// If Service B is down we degrade gracefully (FALLBACK): the stored booking is
// still returned, flagged with telescope_details_available=false.
func (srv *Server) GetSession(ctx context.Context, req *sessionpb.GetSessionRequest) (*sessionpb.Session, error) {
	if err := ValidateID(req.GetId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	sess, err := srv.repo.Get(ctx, req.GetId())
	if err != nil {
		return nil, repoError("get session", err)
	}
	out := toProto(sess)
	srv.enrich(ctx, out, nil)
	return out, nil
}

func (srv *Server) ListSessions(ctx context.Context, req *sessionpb.ListSessionsRequest) (*sessionpb.ListSessionsResponse, error) {
	filter := strings.TrimSpace(req.GetTelescopeId())
	if filter != "" {
		if _, err := uuid.Parse(filter); err != nil {
			return nil, status.Error(codes.InvalidArgument, "telescope_id must be a valid UUID")
		}
	}
	items, err := srv.repo.List(ctx, filter)
	if err != nil {
		return nil, repoError("list sessions", err)
	}

	// One lookup per distinct telescope, not per session.
	cache := map[string]*telescopepb.Telescope{}
	resp := &sessionpb.ListSessionsResponse{Sessions: make([]*sessionpb.Session, 0, len(items))}
	for _, sess := range items {
		out := toProto(sess)
		srv.enrich(ctx, out, cache)
		resp.Sessions = append(resp.Sessions, out)
	}
	return resp, nil
}

// UpdateSession has PUT semantics. Service B is only consulted when the
// booking is moved to a different telescope.
func (srv *Server) UpdateSession(ctx context.Context, req *sessionpb.UpdateSessionRequest) (*sessionpb.Session, error) {
	if err := ValidateID(req.GetId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	sess := Session{
		ID:          req.GetId(),
		TelescopeID: req.GetTelescopeId(),
		Observer:    req.GetObserver(),
		Date:        req.GetDate(),
		Target:      req.GetTarget(),
		Status:      req.GetStatus(),
	}
	sess.Normalize()
	if err := sess.Validate(false, time.Now()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	existing, err := srv.repo.Get(ctx, sess.ID)
	if err != nil {
		return nil, repoError("update session", err)
	}
	if sess.TelescopeID != existing.TelescopeID {
		if _, err := srv.lookupBookable(ctx, sess.TelescopeID); err != nil {
			return nil, err
		}
	}
	if err := srv.repo.Update(ctx, sess); err != nil {
		return nil, repoError("update session", err)
	}

	out := toProto(sess)
	srv.enrich(ctx, out, nil)
	return out, nil
}

func (srv *Server) DeleteSession(ctx context.Context, req *sessionpb.DeleteSessionRequest) (*sessionpb.DeleteSessionResponse, error) {
	if err := ValidateID(req.GetId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := srv.repo.Delete(ctx, req.GetId()); err != nil {
		return nil, repoError("delete session", err)
	}
	return &sessionpb.DeleteSessionResponse{}, nil
}

// lookupBookable asks Service B for the telescope and applies the business
// rule "only AVAILABLE telescopes can be booked".
func (srv *Server) lookupBookable(ctx context.Context, telescopeID string) (*telescopepb.Telescope, error) {
	tel, err := srv.telescopes.GetTelescope(ctx, telescopeID)
	if err != nil {
		return nil, translateTelescopeError(err, telescopeID)
	}
	if tel.GetStatus() != telescopeAvailable {
		return nil, status.Errorf(codes.FailedPrecondition,
			"telescope %q is %s and cannot be booked", tel.GetName(), strings.ToLower(tel.GetStatus()))
	}
	return tel, nil
}

// enrich fills telescope_name from Service B, or falls back gracefully.
func (srv *Server) enrich(ctx context.Context, out *sessionpb.Session, cache map[string]*telescopepb.Telescope) {
	if tel, ok := cache[out.GetTelescopeId()]; ok {
		applyTelescope(out, tel)
		return
	}
	tel, err := srv.telescopes.GetTelescope(ctx, out.GetTelescopeId())
	if err != nil {
		log.Printf("fallback: telescope details unavailable for session %s: %v", out.GetId(), err)
		tel = nil
	}
	if cache != nil {
		cache[out.GetTelescopeId()] = tel
	}
	applyTelescope(out, tel)
}

func applyTelescope(out *sessionpb.Session, tel *telescopepb.Telescope) {
	if tel == nil {
		out.TelescopeName = ""
		out.TelescopeDetailsAvailable = false
		return
	}
	out.TelescopeName = tel.GetName()
	out.TelescopeDetailsAvailable = true
}

// translateTelescopeError converts what Service B (or the resilience wrapper)
// returned into the error that Service A's own callers should see.
func translateTelescopeError(err error, telescopeID string) error {
	switch status.Code(err) {
	case codes.NotFound:
		return status.Errorf(codes.NotFound, "telescope %q does not exist", telescopeID)
	case codes.InvalidArgument:
		return status.Error(codes.InvalidArgument, "telescope_id is invalid")
	case codes.Unavailable, codes.DeadlineExceeded:
		return status.Error(codes.Unavailable, "telescope service is unavailable, please try again later")
	}
	log.Printf("unexpected error from telescope-service: %v", err)
	return status.Error(codes.Internal, "could not verify telescope")
}

func toProto(s Session) *sessionpb.Session {
	return &sessionpb.Session{
		Id:          s.ID,
		TelescopeId: s.TelescopeID,
		Observer:    s.Observer,
		Date:        s.Date,
		Target:      s.Target,
		Status:      s.Status,
	}
}

func repoError(op string, err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return status.Error(codes.NotFound, "session not found")
	case errors.Is(err, ErrSlotTaken):
		return status.Error(codes.AlreadyExists, "that telescope is already booked on this date")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request timed out")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request cancelled")
	}
	log.Printf("%s failed: %v", op, err)
	return status.Error(codes.Internal, "internal error")
}
