package service

import "treino/internal/domain"

func (s *Service) Save1RM(athleteID, exerciseID int, weightKg float64) error {
	return s.repo.Save1RM(athleteID, exerciseID, weightKg)
}

func (s *Service) List1RMs(athleteID int) ([]domain.OneRM, error) {
	return s.repo.List1RMs(athleteID)
}
