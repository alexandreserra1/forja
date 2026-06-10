package handler

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"treino/internal/service"
)

type ctxKey int

const ctxAthleteID ctxKey = 0

// RequireAuth valida o Bearer token JWT e injeta o athleteID no contexto.
// Rotas públicas (/api/auth/*) devem ser registradas FORA deste middleware.
//
// Dev mode: se AUTH_SECRET não estiver definida no ambiente, requisições sem
// Bearer token passam sem autenticação (para facilitar testes locais).
// Em produção, AUTH_SECRET DEVE estar definida — sem ela o JWT não é válido.
func RequireAuth(svc *service.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			raw := r.Header.Get("Authorization")
			token, hasBearer := strings.CutPrefix(raw, "Bearer ")
			hasBearer = hasBearer && token != ""

			if hasBearer {
				athleteID, err := svc.ValidateToken(token)
				if err != nil {
					writeError(w, http.StatusUnauthorized, errUnauthorized)
					return
				}
				ctx := context.WithValue(r.Context(), ctxAthleteID, athleteID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Sem token: em dev (AUTH_SECRET vazia) passa; em prod retorna 401.
			if os.Getenv("AUTH_SECRET") != "" {
				writeError(w, http.StatusUnauthorized, errUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// athleteIDFromCtx lê o athleteID injetado pelo RequireAuth.
// Retorna 1 como fallback para manter compatibilidade com testes que não passam pelo middleware.
func athleteIDFromCtx(r *http.Request) int {
	if id, ok := r.Context().Value(ctxAthleteID).(int); ok && id > 0 {
		return id
	}
	return 1
}

var errUnauthorized = fmt.Errorf("não autorizado")
