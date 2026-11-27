// Package auth provides authentication context and RBAC contracts.
package auth

import (
	"context"

	"github.com/google/uuid"
)

// Role constants matching seeded roles in PostgreSQL.
const (
	RoleDoctor     = "doctor"
	RolePharmacist = "pharmacist"
	RolePatient    = "patient"
	RoleAdmin      = "admin"
)

// ActorContext represents the authenticated caller stored in request context.
type ActorContext struct {
	UserID uuid.UUID
	IIN    string
	Role   string
}

type actorCtxKey struct{}

// WithActor attaches an ActorContext to ctx.
func WithActor(ctx context.Context, actor ActorContext) context.Context {
	return context.WithValue(ctx, actorCtxKey{}, actor)
}

// ActorFromContext extracts the authenticated ActorContext from ctx, if present.
func ActorFromContext(ctx context.Context) (ActorContext, bool) {
	actor, ok := ctx.Value(actorCtxKey{}).(ActorContext)
	if !ok || actor.UserID == uuid.Nil {
		return ActorContext{}, false
	}
	return actor, true
}
