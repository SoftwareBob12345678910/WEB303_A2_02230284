package telescope

import (
	"context"
	"errors"
	"log"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	telescopepb "stargazer/gen/telescope/v1"
)

// Server implements the TelescopeService gRPC API.
// Every error leaves this layer as a proper gRPC status code.
type Server struct {
	telescopepb.UnimplementedTelescopeServiceServer
	repo *Repository
}

func NewServer(repo *Repository) *Server { return &Server{repo: repo} }

func (s *Server) CreateTelescope(ctx context.Context, req *telescopepb.CreateTelescopeRequest) (*telescopepb.Telescope, error) {
	t := Telescope{
		ID:         uuid.NewString(),
		Name:       req.GetName(),
		ApertureMM: req.GetApertureMm(),
		Type:       req.GetType(),
		Status:     req.GetStatus(),
	}
	t.Normalize()
	if t.Status == "" {
		t.Status = StatusAvailable
	}
	if err := t.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, internalError("create telescope", err)
	}
	return toProto(t), nil
}

func (s *Server) GetTelescope(ctx context.Context, req *telescopepb.GetTelescopeRequest) (*telescopepb.Telescope, error) {
	if err := ValidateID(req.GetId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	t, err := s.repo.Get(ctx, req.GetId())
	if err != nil {
		return nil, repoError("get telescope", err)
	}
	return toProto(t), nil
}

func (s *Server) ListTelescopes(ctx context.Context, _ *telescopepb.ListTelescopesRequest) (*telescopepb.ListTelescopesResponse, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, internalError("list telescopes", err)
	}
	resp := &telescopepb.ListTelescopesResponse{Telescopes: make([]*telescopepb.Telescope, 0, len(items))}
	for _, t := range items {
		resp.Telescopes = append(resp.Telescopes, toProto(t))
	}
	return resp, nil
}

// UpdateTelescope has PUT semantics: every field is replaced and must be valid.
func (s *Server) UpdateTelescope(ctx context.Context, req *telescopepb.UpdateTelescopeRequest) (*telescopepb.Telescope, error) {
	if err := ValidateID(req.GetId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	t := Telescope{
		ID:         req.GetId(),
		Name:       req.GetName(),
		ApertureMM: req.GetApertureMm(),
		Type:       req.GetType(),
		Status:     req.GetStatus(),
	}
	t.Normalize()
	if err := t.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.repo.Update(ctx, t); err != nil {
		return nil, repoError("update telescope", err)
	}
	return toProto(t), nil
}

func (s *Server) DeleteTelescope(ctx context.Context, req *telescopepb.DeleteTelescopeRequest) (*telescopepb.DeleteTelescopeResponse, error) {
	if err := ValidateID(req.GetId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.repo.Delete(ctx, req.GetId()); err != nil {
		return nil, repoError("delete telescope", err)
	}
	return &telescopepb.DeleteTelescopeResponse{}, nil
}

func toProto(t Telescope) *telescopepb.Telescope {
	return &telescopepb.Telescope{
		Id:         t.ID,
		Name:       t.Name,
		ApertureMm: t.ApertureMM,
		Type:       t.Type,
		Status:     t.Status,
	}
}

// repoError maps repository errors to gRPC status codes.
func repoError(op string, err error) error {
	if errors.Is(err, ErrNotFound) {
		return status.Error(codes.NotFound, "telescope not found")
	}
	return internalError(op, err)
}

// internalError logs the real cause but never leaks it to the caller.
func internalError(op string, err error) error {
	log.Printf("%s failed: %v", op, err)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request timed out")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request cancelled")
	}
	return status.Error(codes.Internal, "internal error")
}
