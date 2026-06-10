package service

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"treino/internal/domain"
	"treino/internal/repository"
)

const bcryptCost = 12

// jwtKey é lida uma vez do ambiente. Fatal no boot se ausente — erro de configuração,
// não de runtime; melhor falhar cedo do que assinar tokens com chave vazia.
var jwtKey = func() []byte {
	k := os.Getenv("AUTH_SECRET")
	if k == "" {
		// Em dev local (sem variável) usamos uma chave fixa e avisamos.
		// Em produção o servidor deve setar AUTH_SECRET e o aviso vira erro.
		fmt.Fprintln(os.Stderr, "AVISO: AUTH_SECRET não definido — usando chave de desenvolvimento. NÃO use em produção.")
		k = "cfit-dev-secret-change-in-production"
	}
	return []byte(k)
}()

var ErrInvalidCredentials = errors.New("e-mail ou senha inválidos")

// Register cria um atleta e grava suas credenciais. Devolve o atleta + JWT.
func (s *Service) Register(name, email, password string) (*domain.Athlete, string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, "", fmt.Errorf("hash senha: %w", err)
	}

	athlete, err := s.repo.CreateAthlete(name)
	if err != nil {
		return nil, "", err
	}

	if err := s.repo.CreateAuth(athlete.ID, email, string(hash)); err != nil {
		return nil, "", err
	}

	token, err := signToken(athlete.ID)
	if err != nil {
		return nil, "", err
	}
	return athlete, token, nil
}

// Login valida as credenciais e devolve o atleta + JWT.
func (s *Service) Login(email, password string) (*domain.Athlete, string, error) {
	auth, err := s.repo.GetAuthByEmail(email)
	if err != nil {
		return nil, "", err
	}
	if auth == nil {
		return nil, "", ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(auth.PasswordHash), []byte(password)); err != nil {
		return nil, "", ErrInvalidCredentials
	}

	athletes, err := s.repo.ListAthletes()
	if err != nil {
		return nil, "", err
	}
	var athlete *domain.Athlete
	for i := range athletes {
		if athletes[i].ID == auth.AthleteID {
			athlete = &athletes[i]
			break
		}
	}
	if athlete == nil {
		return nil, "", fmt.Errorf("atleta %d não encontrado", auth.AthleteID)
	}

	token, err := signToken(athlete.ID)
	if err != nil {
		return nil, "", err
	}
	return athlete, token, nil
}

// ValidateToken valida o JWT e devolve o athleteID. Retorna erro se inválido ou expirado.
func (s *Service) ValidateToken(tokenStr string) (int, error) {
	tok, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("algoritmo inesperado: %v", t.Header["alg"])
		}
		return jwtKey, nil
	}, jwt.WithExpirationRequired())
	if err != nil {
		return 0, err
	}

	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return 0, fmt.Errorf("claims inválidas")
	}
	sub, err := claims.GetSubject()
	if err != nil || sub == "" {
		return 0, fmt.Errorf("subject ausente")
	}

	var athleteID int
	if _, err := fmt.Sscan(sub, &athleteID); err != nil || athleteID <= 0 {
		return 0, fmt.Errorf("subject inválido: %q", sub)
	}
	return athleteID, nil
}

func signToken(athleteID int) (string, error) {
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   fmt.Sprintf("%d", athleteID),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
	})
	return tok.SignedString(jwtKey)
}

// ErrEmailTaken re-exporta o erro do repo para o handler poder verificar sem importar o repo.
var ErrEmailTaken = repository.ErrEmailTaken
