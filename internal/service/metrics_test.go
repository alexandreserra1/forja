package service

import (
	"testing"

	"treino/internal/domain"
)

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }

func TestVolumeFactor_Nil(t *testing.T) {
	s, c := VolumeFactor(nil)
	if s != 1.0 || c != 1.0 {
		t.Fatalf("nil deveria retornar 1.0, 1.0; veio %v, %v", s, c)
	}
}

func TestVolumeFactor_Idade(t *testing.T) {
	casos := []struct {
		age             int
		wantS, wantC float64
	}{
		{25, 1.0, 1.0},
		{30, 0.95, 0.95},
		{39, 0.95, 0.95},
		{40, 0.90, 0.90},
		{49, 0.90, 0.90},
		{50, 0.85, 0.85},
		{65, 0.85, 0.85},
	}
	for _, tc := range casos {
		s, c := VolumeFactor(&domain.AthleteMetrics{AgeYears: intPtr(tc.age)})
		if s != tc.wantS || c != tc.wantC {
			t.Errorf("idade %d: esperava (%.2f,%.2f) veio (%.2f,%.2f)", tc.age, tc.wantS, tc.wantC, s, c)
		}
	}
}

func TestVolumeFactor_Esporte(t *testing.T) {
	casos := []struct {
		sport           string
		wantS, wantC float64
	}{
		{"crossfit", 1.0, 1.0},
		{"general_fitness", 1.0, 1.0},
		{"weightlifting", 1.10, 0.90},
		{"endurance", 0.90, 1.10},
	}
	for _, tc := range casos {
		s, c := VolumeFactor(&domain.AthleteMetrics{Sport: strPtr(tc.sport)})
		if abs(s-tc.wantS) > 0.001 || abs(c-tc.wantC) > 0.001 {
			t.Errorf("sport %s: esperava (%.2f,%.2f) veio (%.2f,%.2f)", tc.sport, tc.wantS, tc.wantC, s, c)
		}
	}
}

func TestVolumeFactor_Combinado(t *testing.T) {
	// 45 anos weightlifter: força=0.90*1.10=0.99, cond=0.90*0.90=0.81
	s, c := VolumeFactor(&domain.AthleteMetrics{
		AgeYears: intPtr(45),
		Sport:    strPtr("weightlifting"),
	})
	if abs(s-0.99) > 0.001 || abs(c-0.81) > 0.001 {
		t.Errorf("combinado: esperava (0.99,0.81) veio (%.3f,%.3f)", s, c)
	}
}

func TestApplyFactor(t *testing.T) {
	if applyFactor(4, 1.10) != 4 { // 4.4 arredonda p/ 4
		t.Error("4*1.10 deve ser 4")
	}
	if applyFactor(4, 0.90) != 4 { // 3.6 arredonda p/ 4
		t.Error("4*0.90 deve ser 4")
	}
	if applyFactor(5, 1.10) != 6 { // 5.5 arredonda p/ 6
		t.Error("5*1.10 deve ser 6")
	}
	if applyFactor(0, 1.0) != 1 { // mínimo 1
		t.Error("mínimo deve ser 1")
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
