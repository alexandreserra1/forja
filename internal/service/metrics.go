package service

import "treino/internal/domain"

// VolumeFactor calcula os multiplicadores de dose para FORÇA (strength) e CONDICIONAMENTO (cond)
// a partir das métricas do atleta. Todos os fatores são PLACEHOLDERS CALIBRÁVEIS — devem ser
// validados com treinador antes de usuário real.
//
// Regras (nil = ausente = 1.0):
//   - Idade: <30→1.0; 30–39→0.95; 40–49→0.90; 50+→0.85
//   - Esporte: weightlifting → força+10%, cond−10%
//              endurance     → força−10%, cond+10%
//              crossfit / general_fitness / nil → 1.0
//   - Sexo / peso: não alteram volume na v1 (dívida declarada)
func VolumeFactor(m *domain.AthleteMetrics) (strength, cond float64) {
	strength, cond = 1.0, 1.0
	if m == nil {
		return
	}

	// Fator por idade.
	if m.AgeYears != nil {
		switch {
		case *m.AgeYears >= 50:
			strength *= 0.85
			cond *= 0.85
		case *m.AgeYears >= 40:
			strength *= 0.90
			cond *= 0.90
		case *m.AgeYears >= 30:
			strength *= 0.95
			cond *= 0.95
		}
	}

	// Fator por esporte.
	if m.Sport != nil {
		switch *m.Sport {
		case "weightlifting":
			strength *= 1.10
			cond *= 0.90
		case "endurance":
			strength *= 0.90
			cond *= 1.10
		}
		// crossfit / general_fitness / outros: sem alteração
	}

	return
}

// applyFactor aplica um fator float a um inteiro, retornando mínimo 1.
func applyFactor(n int, factor float64) int {
	v := int(float64(n)*factor + 0.5) // arredonda
	if v < 1 {
		return 1
	}
	return v
}
