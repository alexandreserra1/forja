-- ============================================================
-- treino / cfit — seed da Fase 0
-- ============================================================

-- Perguntas básicas
INSERT INTO question (text, type, sort_order, show_when) VALUES
  ('Há quanto tempo você treina?', 'single_choice', 1, NULL),
  ('Quantos dias por semana você quer treinar?', 'single_choice', 2, NULL),
  ('Qual seu principal objetivo?', 'single_choice', 3, NULL);

-- Opções da pergunta 1 (tempo de treino)
INSERT INTO question_option (question_id, label, value, sort_order) VALUES
  (1, 'Menos de 1 ano', 'lt_1y', 1),
  (1, '1 a 3 anos', '1_3y', 2),
  (1, 'Mais de 3 anos', 'gt_3y', 3);

-- Opções da pergunta 2 (dias)
INSERT INTO question_option (question_id, label, value, sort_order) VALUES
  (2, '3 dias', '3', 1),
  (2, '4 dias', '4', 2);

-- Opções da pergunta 3 (objetivo)
-- FIX: 'Técnica' tinha sort_order = 1 (colidia com 'Força'). Corrigido para 3.
INSERT INTO question_option (question_id, label, value, sort_order) VALUES
  (3, 'Força', 'strength', 1),
  (3, 'Condicionamento', 'conditioning', 2),
  (3, 'Técnica', 'technique', 3);

-- Interpretações: resposta -> atributo
INSERT INTO answer_rule (question_id, option_value, sets_attribute, attribute_value) VALUES
  (1, 'lt_1y', 'level', 'beginner'),
  (1, '1_3y',  'level', 'intermediate'),
  (1, 'gt_3y', 'level', 'advanced'),
  (2, '3',     'days',  '3'),
  (2, '4',     'days',  '4'),
  (3, 'strength',     'goal', 'strength'),
  (3, 'conditioning', 'goal', 'conditioning'),
  (3, 'technique',    'goal', 'technique');

-- Padrões de movimento
INSERT INTO movement_pattern (name) VALUES ('squat'), ('hinge'), ('push'), ('pull');

-- Exercícios (alguns por nível/foco para o motor mínimo ter o que escolher).
-- Nota: a cobertura advanced+strength é garantida no motor pela regra
-- "level do usuário E ABAIXO" (Fase 0, M4) — advanced enxerga intermediate/beginner.
INSERT INTO exercise (name, movement_pattern_id, level, focus) VALUES
  ('Air Squat',        1, 'beginner',     'technique'),
  ('Back Squat',       1, 'intermediate', 'strength'),
  ('Overhead Squat',   1, 'advanced',     'technique'),
  ('Deadlift',         2, 'intermediate', 'strength'),
  ('Kettlebell Swing', 2, 'beginner',     'conditioning'),
  ('Push-up',          3, 'beginner',     'technique'),
  ('Strict Press',     3, 'intermediate', 'strength'),
  ('Ring Row',         4, 'beginner',     'technique'),
  ('Pull-up',          4, 'intermediate', 'strength');
