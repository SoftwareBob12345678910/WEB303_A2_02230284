// Package gateway is the single REST entry point. It translates JSON/HTTP
// requests into gRPC calls and gRPC errors back into HTTP status codes.
// It contains no business logic and no database access.
package gateway

import (
	"context"
	"net/http"
	"time"

	sessionpb "stargazer/gen/session/v1"
	telescopepb "stargazer/gen/telescope/v1"
)

type Gateway struct {
	telescopes telescopepb.TelescopeServiceClient
	sessions   sessionpb.SessionServiceClient
	timeout    time.Duration // upper bound for one backend call
}

func New(t telescopepb.TelescopeServiceClient, s sessionpb.SessionServiceClient, timeout time.Duration) *Gateway {
	return &Gateway{telescopes: t, sessions: s, timeout: timeout}
}

// Routes registers resource-based URLs (Go 1.22 method+path patterns).
// A known path with the wrong method automatically gets 405.
func (g *Gateway) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeValue(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /telescopes", g.createTelescope)
	mux.HandleFunc("GET /telescopes", g.listTelescopes)
	mux.HandleFunc("GET /telescopes/{id}", g.getTelescope)
	mux.HandleFunc("PUT /telescopes/{id}", g.updateTelescope)
	mux.HandleFunc("DELETE /telescopes/{id}", g.deleteTelescope)

	mux.HandleFunc("POST /sessions", g.createSession)
	mux.HandleFunc("GET /sessions", g.listSessions)
	mux.HandleFunc("GET /sessions/{id}", g.getSession)
	mux.HandleFunc("PUT /sessions/{id}", g.updateSession)
	mux.HandleFunc("DELETE /sessions/{id}", g.deleteSession)

	return logRequests(mux)
}

func (g *Gateway) callCtx(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), g.timeout)
}

// ---- telescopes -----------------------------------------------------------

func (g *Gateway) createTelescope(w http.ResponseWriter, r *http.Request) {
	var req telescopepb.CreateTelescopeRequest
	if !readJSON(w, r, &req) {
		return
	}
	ctx, cancel := g.callCtx(r)
	defer cancel()
	res, err := g.telescopes.CreateTelescope(ctx, &req)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Location", "/telescopes/"+res.GetId())
	writeProto(w, http.StatusCreated, res)
}

func (g *Gateway) listTelescopes(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := g.callCtx(r)
	defer cancel()
	res, err := g.telescopes.ListTelescopes(ctx, &telescopepb.ListTelescopesRequest{})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, http.StatusOK, res)
}

func (g *Gateway) getTelescope(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := g.callCtx(r)
	defer cancel()
	res, err := g.telescopes.GetTelescope(ctx, &telescopepb.GetTelescopeRequest{Id: r.PathValue("id")})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, http.StatusOK, res)
}

func (g *Gateway) updateTelescope(w http.ResponseWriter, r *http.Request) {
	var req telescopepb.UpdateTelescopeRequest
	if !readJSON(w, r, &req) {
		return
	}
	req.Id = r.PathValue("id") // the URL is the source of truth for the id
	ctx, cancel := g.callCtx(r)
	defer cancel()
	res, err := g.telescopes.UpdateTelescope(ctx, &req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, http.StatusOK, res)
}

func (g *Gateway) deleteTelescope(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := g.callCtx(r)
	defer cancel()
	if _, err := g.telescopes.DeleteTelescope(ctx, &telescopepb.DeleteTelescopeRequest{Id: r.PathValue("id")}); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- sessions -------------------------------------------------------------

func (g *Gateway) createSession(w http.ResponseWriter, r *http.Request) {
	var req sessionpb.CreateSessionRequest
	if !readJSON(w, r, &req) {
		return
	}
	ctx, cancel := g.callCtx(r)
	defer cancel()
	res, err := g.sessions.CreateSession(ctx, &req)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Location", "/sessions/"+res.GetId())
	writeProto(w, http.StatusCreated, res)
}

func (g *Gateway) listSessions(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := g.callCtx(r)
	defer cancel()
	res, err := g.sessions.ListSessions(ctx, &sessionpb.ListSessionsRequest{
		TelescopeId: r.URL.Query().Get("telescope_id"),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, http.StatusOK, res)
}

func (g *Gateway) getSession(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := g.callCtx(r)
	defer cancel()
	res, err := g.sessions.GetSession(ctx, &sessionpb.GetSessionRequest{Id: r.PathValue("id")})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, http.StatusOK, res)
}

func (g *Gateway) updateSession(w http.ResponseWriter, r *http.Request) {
	var req sessionpb.UpdateSessionRequest
	if !readJSON(w, r, &req) {
		return
	}
	req.Id = r.PathValue("id")
	ctx, cancel := g.callCtx(r)
	defer cancel()
	res, err := g.sessions.UpdateSession(ctx, &req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, http.StatusOK, res)
}

func (g *Gateway) deleteSession(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := g.callCtx(r)
	defer cancel()
	if _, err := g.sessions.DeleteSession(ctx, &sessionpb.DeleteSessionRequest{Id: r.PathValue("id")}); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
