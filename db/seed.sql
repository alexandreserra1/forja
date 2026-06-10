-- ============================================================
-- treino / cfit — seed da Fase 0
-- ============================================================

-- FASE 6A: o atleta default (id 1) = o atleta implícito das fases anteriores. As tabelas de estado
-- usam DEFAULT 1, então os fluxos atuais continuam gravando para este atleta sem mudança.
INSERT INTO athlete (id, name) VALUES (1, 'Atleta 1');

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

-- ============================================================
-- FASE 1 — seed
-- ============================================================

-- 4ª pergunta: duração do bloco (vira o atributo 'weeks' do perfil).
INSERT INTO question (text, type, sort_order, show_when) VALUES
  ('Quantas semanas de treino?', 'single_choice', 4, NULL);

-- Opções da pergunta 4 (id = 4)
INSERT INTO question_option (question_id, label, value, sort_order) VALUES
  (4, '8 semanas',  '8',  1),
  (4, '10 semanas', '10', 2),
  (4, '12 semanas', '12', 3);

-- Interpretação: resposta -> atributo 'weeks'
INSERT INTO answer_rule (question_id, option_value, sets_attribute, attribute_value) VALUES
  (4, '8',  'weeks', '8'),
  (4, '10', 'weeks', '10'),
  (4, '12', 'weeks', '12');

-- Moldes de fase para goal='strength'.
-- PLACEHOLDER FUNDAMENTADO — refinar com pesquisa antes de tratar como definitivo (ver plan.md).
-- deload não usa week_share: o motor sempre reserva a última semana como deload.
INSERT INTO phase_template (goal, phase, week_share, base_rpe, rpe_step, default_sets, default_reps, sort_order) VALUES
  ('strength', 'accumulation',    0.50, 6.5, 0.25, 4, 6, 1),
  ('strength', 'intensification', 0.33, 8.0, 0.25, 4, 4, 2),
  ('strength', 'realization',     0.17, 9.0, 0.00, 3, 3, 3),
  ('strength', 'deload',          0.00, 5.0, 0.00, 3, 5, 4);

-- ============================================================
-- FASE 3 — seed (catálogo ENXUTO de propósito; LPO/técnicos numa rodada de conteúdo à parte)
-- ============================================================

-- Catálogo de equipamentos (ids: 1=Barra, 2=Rack, 3=Kettlebell, 4=Argolas, 5=Barra fixa).
INSERT INTO equipment (name) VALUES
  ('Barra'), ('Rack'), ('Kettlebell'), ('Argolas'), ('Barra fixa');

-- O que cada exercício EXIGE. Exercício SEM linha aqui = peso do corpo (sempre disponível):
-- Air Squat (1) e Push-up (6) não aparecem de propósito.
INSERT INTO exercise_equipment (exercise_id, equipment_id) VALUES
  (2, 1), (2, 2),   -- Back Squat: Barra + Rack
  (3, 1),           -- Overhead Squat: Barra
  (4, 1),           -- Deadlift: Barra
  (5, 3),           -- Kettlebell Swing: Kettlebell
  (7, 1),           -- Strict Press: Barra
  (8, 4),           -- Ring Row: Argolas
  (9, 5);           -- Pull-up: Barra fixa

-- Regra ESPECÍFICA de substituição (placeholder fundamentado): no padrão squat, faltando RACK,
-- preferir Overhead Squat (só exige Barra) em vez do que a busca genérica pegaria.
-- phase = NULL: vale para qualquer fase (a Fase 3 ainda não honra fase — ver plan.md).
INSERT INTO substitution_rule (movement_pattern_id, phase, missing_equipment_id, substitute_exercise_id) VALUES
  (1, NULL, 2, 3);  -- squat + falta Rack -> Overhead Squat

-- ============================================================
-- FASE 4 — ETAPA A: catálogo de verdade (PLACEHOLDER FUNDAMENTADO).
-- Conteúdo é DADO calibrável (nível/foco/padrão classificados por conhecimento de treino,
-- ajustáveis por um treinador depois). Pesquisa: variações de LPO (full/power/hang/tall/high-pull,
-- balance) e formatos de condicionamento. Os testes validam a LÓGICA de seleção, não esta lista.
-- Inserts por NOME (CTE + join) para não depender de ids manuais.
-- ============================================================

-- Novos padrões de movimento e equipamentos.
INSERT INTO movement_pattern (name) VALUES ('olympic'), ('lunge'), ('carry'), ('core');
INSERT INTO equipment (name) VALUES ('Halter'), ('Banco'), ('Caixa'), ('Plataforma'), ('Bola');

-- Exercícios (name, padrão, nível, foco). focus ∈ technique|strength|conditioning.
WITH v(name, pattern, level, focus) AS (VALUES
  -- SQUAT
  ('Front Squat',          'squat',   'intermediate', 'strength'),
  ('Box Squat',            'squat',   'intermediate', 'strength'),
  ('Goblet Squat',         'squat',   'beginner',     'strength'),
  ('Pause Squat',          'squat',   'advanced',     'strength'),
  ('Tempo Squat',          'squat',   'intermediate', 'technique'),
  ('Pistol Squat',         'squat',   'advanced',     'technique'),
  ('Zercher Squat',        'squat',   'advanced',     'strength'),
  ('Snatch Balance',       'squat',   'advanced',     'technique'),
  ('Drop Snatch',          'squat',   'advanced',     'technique'),
  ('Wall Ball',            'squat',   'beginner',     'conditioning'),
  ('Thruster',             'squat',   'intermediate', 'conditioning'),
  ('Box Jump',             'squat',   'beginner',     'conditioning'),
  -- HINGE
  ('Romanian Deadlift',    'hinge',   'intermediate', 'strength'),
  ('Sumo Deadlift',        'hinge',   'intermediate', 'strength'),
  ('Deficit Deadlift',     'hinge',   'advanced',     'technique'),
  ('Good Morning',         'hinge',   'advanced',     'strength'),
  ('Snatch-grip Deadlift', 'hinge',   'advanced',     'technique'),
  ('Kettlebell Deadlift',  'hinge',   'beginner',     'technique'),
  ('Hip Thrust',           'hinge',   'intermediate', 'strength'),
  ('Clean Pull',           'hinge',   'intermediate', 'technique'),
  ('Snatch Pull',          'hinge',   'intermediate', 'technique'),
  ('Single-leg RDL',       'hinge',   'intermediate', 'technique'),
  ('Back Extension',       'hinge',   'beginner',     'technique'),
  ('Kettlebell Clean',     'hinge',   'beginner',     'conditioning'),
  ('Dumbbell Snatch',      'hinge',   'intermediate', 'conditioning'),
  -- PUSH (empurrada / press / overhead)
  ('Bench Press',          'push',    'intermediate', 'strength'),
  ('Incline Bench Press',  'push',    'intermediate', 'strength'),
  ('DB Bench Press',       'push',    'beginner',     'strength'),
  ('Push Press',           'push',    'intermediate', 'strength'),
  ('Push Jerk',            'push',    'intermediate', 'strength'),
  ('Split Jerk',           'push',    'advanced',     'strength'),
  ('DB Shoulder Press',    'push',    'beginner',     'strength'),
  ('Z Press',              'push',    'intermediate', 'technique'),
  ('Behind-the-neck Press','push',    'advanced',     'technique'),
  ('Pike Push-up',         'push',    'intermediate', 'technique'),
  ('Handstand Hold',       'push',    'advanced',     'technique'),
  ('Handstand Push-up',    'push',    'advanced',     'strength'),
  ('Ring Dip',             'push',    'intermediate', 'strength'),
  ('Burpee',               'push',    'beginner',     'conditioning'),
  -- PULL (puxada)
  ('Bent-over Row',        'pull',    'intermediate', 'strength'),
  ('Pendlay Row',          'pull',    'intermediate', 'strength'),
  ('Dumbbell Row',         'pull',    'beginner',     'strength'),
  ('Inverted Row',         'pull',    'beginner',     'technique'),
  ('Chin-up',              'pull',    'intermediate', 'strength'),
  ('Scapular Pull-up',     'pull',    'beginner',     'technique'),
  ('Chest-to-bar Pull-up', 'pull',    'advanced',     'strength'),
  ('Weighted Pull-up',     'pull',    'advanced',     'strength'),
  ('Snatch-grip Pull-up',  'pull',    'advanced',     'technique'),
  ('Bar Muscle-up',        'pull',    'advanced',     'strength'),
  ('Ring Muscle-up',       'pull',    'advanced',     'strength'),
  ('Face Pull',            'pull',    'beginner',     'technique'),
  -- OLYMPIC (LPO completo: full/power/hang/tall/high-pull/block)
  ('Power Clean',          'olympic', 'intermediate', 'strength'),
  ('Power Snatch',         'olympic', 'intermediate', 'strength'),
  ('Hang Power Clean',     'olympic', 'intermediate', 'technique'),
  ('Hang Power Snatch',    'olympic', 'intermediate', 'technique'),
  ('Clean',                'olympic', 'advanced',     'strength'),
  ('Snatch',               'olympic', 'advanced',     'strength'),
  ('Clean and Jerk',       'olympic', 'advanced',     'strength'),
  ('Hang Snatch',          'olympic', 'advanced',     'technique'),
  ('Tall Snatch',          'olympic', 'advanced',     'technique'),
  ('Tall Clean',           'olympic', 'advanced',     'technique'),
  ('Clean High Pull',      'olympic', 'intermediate', 'technique'),
  ('Snatch High Pull',     'olympic', 'intermediate', 'technique'),
  ('Muscle Snatch',        'olympic', 'advanced',     'technique'),
  ('Block Clean',          'olympic', 'advanced',     'technique'),
  ('Block Snatch',         'olympic', 'advanced',     'technique'),
  -- LUNGE (unilateral / balance)
  ('Walking Lunge',        'lunge',   'beginner',     'technique'),
  ('Reverse Lunge',        'lunge',   'beginner',     'technique'),
  ('Bulgarian Split Squat','lunge',   'intermediate', 'strength'),
  ('Front Rack Lunge',     'lunge',   'intermediate', 'strength'),
  ('Overhead Lunge',       'lunge',   'advanced',     'technique'),
  ('Step-up',              'lunge',   'beginner',     'technique'),
  ('Cossack Squat',        'lunge',   'intermediate', 'technique'),
  -- CARRY (overhead/balance/grip)
  ('Farmer Carry',         'carry',   'beginner',     'strength'),
  ('Front Rack Carry',     'carry',   'intermediate', 'strength'),
  ('Overhead Carry',       'carry',   'intermediate', 'technique'),
  ('Suitcase Carry',       'carry',   'beginner',     'strength'),
  -- CORE / BALANCE
  ('Plank',                'core',    'beginner',     'technique'),
  ('Hollow Hold',          'core',    'beginner',     'technique'),
  ('L-sit',                'core',    'intermediate', 'technique'),
  ('Hanging Leg Raise',    'core',    'intermediate', 'technique'),
  ('Toes-to-bar',          'core',    'intermediate', 'conditioning'),
  ('Turkish Get-up',       'core',    'advanced',     'technique'),
  ('Pallof Press',         'core',    'beginner',     'technique'),
  ('Ab Wheel Rollout',     'core',    'intermediate', 'technique'),
  ('Single-leg Balance',   'core',    'beginner',     'technique')
)
INSERT INTO exercise (name, movement_pattern_id, level, focus)
SELECT v.name, mp.id, v.level, v.focus FROM v JOIN movement_pattern mp ON mp.name = v.pattern;

-- Equipamento exigido (por NOME). Exercício sem linha aqui = peso do corpo (sempre disponível).
WITH ee(ex, eq) AS (VALUES
  ('Front Squat','Barra'),('Front Squat','Rack'),
  ('Box Squat','Barra'),('Box Squat','Rack'),('Box Squat','Caixa'),
  ('Goblet Squat','Halter'),
  ('Pause Squat','Barra'),('Pause Squat','Rack'),
  ('Tempo Squat','Barra'),('Tempo Squat','Rack'),
  ('Zercher Squat','Barra'),
  ('Snatch Balance','Barra'),
  ('Drop Snatch','Barra'),
  ('Wall Ball','Bola'),
  ('Thruster','Barra'),
  ('Box Jump','Caixa'),
  ('Romanian Deadlift','Barra'),
  ('Sumo Deadlift','Barra'),
  ('Deficit Deadlift','Barra'),('Deficit Deadlift','Plataforma'),
  ('Good Morning','Barra'),('Good Morning','Rack'),
  ('Snatch-grip Deadlift','Barra'),
  ('Kettlebell Deadlift','Kettlebell'),
  ('Hip Thrust','Barra'),('Hip Thrust','Banco'),
  ('Clean Pull','Barra'),('Clean Pull','Plataforma'),
  ('Snatch Pull','Barra'),('Snatch Pull','Plataforma'),
  ('Single-leg RDL','Halter'),
  ('Kettlebell Clean','Kettlebell'),
  ('Dumbbell Snatch','Halter'),
  ('Bench Press','Barra'),('Bench Press','Rack'),('Bench Press','Banco'),
  ('Incline Bench Press','Barra'),('Incline Bench Press','Rack'),('Incline Bench Press','Banco'),
  ('DB Bench Press','Halter'),('DB Bench Press','Banco'),
  ('Push Press','Barra'),
  ('Push Jerk','Barra'),
  ('Split Jerk','Barra'),
  ('DB Shoulder Press','Halter'),
  ('Z Press','Barra'),
  ('Behind-the-neck Press','Barra'),
  ('Ring Dip','Argolas'),
  ('Bent-over Row','Barra'),
  ('Pendlay Row','Barra'),
  ('Dumbbell Row','Halter'),
  ('Inverted Row','Barra fixa'),
  ('Chin-up','Barra fixa'),
  ('Scapular Pull-up','Barra fixa'),
  ('Chest-to-bar Pull-up','Barra fixa'),
  ('Weighted Pull-up','Barra fixa'),
  ('Snatch-grip Pull-up','Barra fixa'),
  ('Bar Muscle-up','Barra fixa'),
  ('Ring Muscle-up','Argolas'),
  ('Face Pull','Halter'),
  ('Power Clean','Barra'),('Power Clean','Plataforma'),
  ('Power Snatch','Barra'),('Power Snatch','Plataforma'),
  ('Hang Power Clean','Barra'),
  ('Hang Power Snatch','Barra'),
  ('Clean','Barra'),('Clean','Plataforma'),
  ('Snatch','Barra'),('Snatch','Plataforma'),
  ('Clean and Jerk','Barra'),('Clean and Jerk','Plataforma'),
  ('Hang Snatch','Barra'),
  ('Tall Snatch','Barra'),
  ('Tall Clean','Barra'),
  ('Clean High Pull','Barra'),
  ('Snatch High Pull','Barra'),
  ('Muscle Snatch','Barra'),
  ('Block Clean','Barra'),('Block Clean','Caixa'),
  ('Block Snatch','Barra'),('Block Snatch','Caixa'),
  ('Bulgarian Split Squat','Halter'),('Bulgarian Split Squat','Banco'),
  ('Front Rack Lunge','Barra'),
  ('Overhead Lunge','Barra'),
  ('Step-up','Caixa'),
  ('Farmer Carry','Halter'),
  ('Front Rack Carry','Barra'),
  ('Overhead Carry','Halter'),
  ('Suitcase Carry','Halter'),
  ('Hanging Leg Raise','Barra fixa'),
  ('Toes-to-bar','Barra fixa'),
  ('Turkish Get-up','Kettlebell'),
  ('Pallof Press','Halter')
)
INSERT INTO exercise_equipment (exercise_id, equipment_id)
SELECT e.id, q.id FROM ee
  JOIN exercise e ON e.name = ee.ex
  JOIN equipment q ON q.name = ee.eq;

-- ============================================================
-- FASE 5A — CONJUGADOS (complexes): sequências de LPO feitas como UMA unidade.
-- Placeholder FUNDAMENTADO e calibrável (a escolha dos movimentos e as reps por componente seguem
-- esquemas típicos de LPO, mas são ajustáveis). kind='complex'; cada componente é um exercício REAL
-- do catálogo; o equipamento do conjugado é a UNIÃO dos equipamentos dos componentes (mantém o funil).
-- Inserts por NOME (não por id) p/ resistir a mudanças de ordem do catálogo.
-- ============================================================
WITH c(name, pattern, level, focus) AS (VALUES
  ('Complexo de Arranco (Snatch Pull + Power Snatch + Overhead Squat)', 'olympic', 'advanced',     'technique'),
  ('Complexo de Arremesso (Clean Pull + Power Clean + Push Jerk)',      'olympic', 'advanced',     'strength'),
  ('Complexo de Potência (Power Clean + Front Squat + Push Press)',     'olympic', 'intermediate', 'strength')
)
INSERT INTO exercise (name, movement_pattern_id, level, focus, kind)
SELECT c.name, mp.id, c.level, c.focus, 'complex'
FROM c JOIN movement_pattern mp ON mp.name = c.pattern;

-- Componentes de cada conjugado, em ordem, com as reps por componente (calibrável).
WITH ci(complex, component, sort_order, reps) AS (VALUES
  ('Complexo de Arranco (Snatch Pull + Power Snatch + Overhead Squat)', 'Snatch Pull',    1, 1),
  ('Complexo de Arranco (Snatch Pull + Power Snatch + Overhead Squat)', 'Power Snatch',   2, 1),
  ('Complexo de Arranco (Snatch Pull + Power Snatch + Overhead Squat)', 'Overhead Squat', 3, 2),
  ('Complexo de Arremesso (Clean Pull + Power Clean + Push Jerk)',      'Clean Pull',     1, 1),
  ('Complexo de Arremesso (Clean Pull + Power Clean + Push Jerk)',      'Power Clean',    2, 1),
  ('Complexo de Arremesso (Clean Pull + Power Clean + Push Jerk)',      'Push Jerk',      3, 1),
  ('Complexo de Potência (Power Clean + Front Squat + Push Press)',     'Power Clean',    1, 1),
  ('Complexo de Potência (Power Clean + Front Squat + Push Press)',     'Front Squat',    2, 2),
  ('Complexo de Potência (Power Clean + Front Squat + Push Press)',     'Push Press',     3, 2)
)
INSERT INTO complex_item (complex_id, component_exercise_id, sort_order, reps)
SELECT cx.id, comp.id, ci.sort_order, ci.reps
FROM ci
  JOIN exercise cx   ON cx.name = ci.complex
  JOIN exercise comp ON comp.name = ci.component;

-- Equipamento do conjugado = UNIÃO dos equipamentos dos componentes (consistência por dado).
--   Snatch Pull/Power Snatch -> Barra+Plataforma; Overhead Squat -> Barra        => Barra, Plataforma
--   Clean Pull/Power Clean   -> Barra+Plataforma; Push Jerk      -> Barra        => Barra, Plataforma
--   Power Clean -> Barra+Plataforma; Front Squat -> Barra+Rack; Push Press -> Barra => Barra, Plataforma, Rack
WITH ce(complex, eq) AS (VALUES
  ('Complexo de Arranco (Snatch Pull + Power Snatch + Overhead Squat)', 'Barra'),
  ('Complexo de Arranco (Snatch Pull + Power Snatch + Overhead Squat)', 'Plataforma'),
  ('Complexo de Arremesso (Clean Pull + Power Clean + Push Jerk)',      'Barra'),
  ('Complexo de Arremesso (Clean Pull + Power Clean + Push Jerk)',      'Plataforma'),
  ('Complexo de Potência (Power Clean + Front Squat + Push Press)',     'Barra'),
  ('Complexo de Potência (Power Clean + Front Squat + Push Press)',     'Plataforma'),
  ('Complexo de Potência (Power Clean + Front Squat + Push Press)',     'Rack')
)
INSERT INTO exercise_equipment (exercise_id, equipment_id)
SELECT cx.id, q.id
FROM ce
  JOIN exercise cx ON cx.name = ce.complex
  JOIN equipment q ON q.name = ce.eq;

-- ============================================================
-- FASE 5B — CONDICIONAMENTO CrossFit (catálogo). Tudo DADO calibrável (placeholder fundamentado):
-- movimentos, formatos, mapa tempo->sistema, doses por fase e WODs concretos. Os testes validam a
-- LÓGICA (duração->sistema, fase->ênfase, equipamento), não os números absolutos.
-- ============================================================

-- Padrão de movimento monoestrutural (corrida/remo/bike/corda) + equipamentos de condicionamento.
INSERT INTO movement_pattern (name) VALUES ('cardio');
INSERT INTO equipment (name) VALUES ('Remador'), ('Corda'), ('Bike');

-- Movimentos monoestruturais NOVOS (focus='conditioning'). Wall Ball/Thruster/Box Jump/Burpee já
-- existem no catálogo da Fase 4 — os WODs abaixo os REUSAM por nome (não reinserimos).
WITH m(name, pattern, level, focus) AS (VALUES
  ('Run',           'cardio', 'beginner',     'conditioning'),
  ('Row',           'cardio', 'beginner',     'conditioning'),
  ('Air Bike',      'cardio', 'beginner',     'conditioning'),
  ('Single Under',  'cardio', 'beginner',     'conditioning'),
  ('Double Under',  'cardio', 'intermediate', 'conditioning'),
  ('Box Jump Over', 'squat',  'intermediate', 'conditioning')
)
INSERT INTO exercise (name, movement_pattern_id, level, focus)
SELECT m.name, mp.id, m.level, m.focus FROM m JOIN movement_pattern mp ON mp.name = m.pattern;

-- Equipamento exigido pelos NOVOS movimentos (Run não exige nada = peso do corpo).
WITH me(ex, eq) AS (VALUES
  ('Row','Remador'),
  ('Air Bike','Bike'),
  ('Single Under','Corda'),
  ('Double Under','Corda'),
  ('Box Jump Over','Caixa')
)
INSERT INTO exercise_equipment (exercise_id, equipment_id)
SELECT e.id, q.id FROM me JOIN exercise e ON e.name = me.ex JOIN equipment q ON q.name = me.eq;

-- Formatos de WOD (duração típica em segundos é placeholder).
INSERT INTO wod_format (name, default_domain_sec) VALUES
  ('AMRAP', 720), ('EMOM', 600), ('ForTime', 300), ('Intervals', 600), ('Chipper', 900);

-- Mapa tempo->sistema ENFATIZADO (fundamentado; faixas calibráveis). Banda = limite superior de
-- work_sec; a primeira banda (ordem crescente) que comporta o work_sec ganha.
INSERT INTO energy_system_map (max_work_sec, system, sort_order) VALUES
  (15,   'phosphagen', 1),   -- < ~15s: potência alática
  (120,  'glycolytic', 2),   -- ~30s-2min: lático
  (480,  'oxidative',  3),   -- ~2-8min sustentado: aeróbio
  (3600, 'mixed',      4);   -- ~8-60min: WOD típico, drift entre sistemas

-- Dose de ênfase por fase (goal='strength', o único com molde de fase). Placeholder calibrável:
-- base aeróbia na acumulação, lático na intensificação, potência na realização, leve no deload.
INSERT INTO phase_conditioning (goal, phase, emphasis_system, wod_target_rpe, weekly_wods, sort_order) VALUES
  ('strength', 'accumulation',    'oxidative',  6.0, 2, 1),
  ('strength', 'intensification', 'glycolytic', 7.5, 2, 2),
  ('strength', 'realization',     'phosphagen', 8.0, 1, 3),
  ('strength', 'deload',          'oxidative',  5.0, 1, 4);

-- WODs placeholder (name, formato, work_sec, rest_sec, rounds, emphasis, target_rpe, level).
-- work_sec é coerente com o emphasis pelo mapa acima (validado no service e no teste).
WITH w(name, format, work_sec, rest_sec, rounds, emphasis, target_rpe, level) AS (VALUES
  -- phosphagen (≤15s de trabalho, descanso longo, intervalado)
  ('Sprint Intervals 10x15s', 'Intervals', 15, 45, 10, 'phosphagen', 8.0, 'beginner'),
  ('Power EMOM Cleans',       'EMOM',      12, 48, 10, 'phosphagen', 8.0, 'intermediate'),
  ('Box Jump Singles',        'Intervals', 10, 50, 10, 'phosphagen', 7.0, 'beginner'),
  -- glycolytic (16-120s, duro, descanso curto)
  ('90s Burpee Blast',        'ForTime',   90,  0, 1, 'glycolytic', 9.0, 'beginner'),
  ('Thruster Sprint',         'ForTime',   75,  0, 1, 'glycolytic', 9.0, 'intermediate'),
  ('Row 500m',                'ForTime',  110,  0, 1, 'glycolytic', 8.0, 'beginner'),
  -- oxidative (121-480s sustentado)
  ('Run 1km Steady',          'ForTime',  300,  0, 1, 'oxidative',  6.0, 'beginner'),
  ('Row 1000m',               'ForTime',  240,  0, 1, 'oxidative',  7.0, 'beginner'),
  ('Air Bike 6min',           'Intervals',360,  0, 1, 'oxidative',  6.0, 'beginner'),
  -- mixed (481-1200s, WOD típico)
  ('Bodyweight AMRAP 12',     'AMRAP',    720,  0, 1, 'mixed',      7.0, 'beginner'),
  ('Chipper 15',              'Chipper',  900,  0, 1, 'mixed',      8.0, 'intermediate'),
  ('Engine AMRAP 20',         'AMRAP',   1200,  0, 1, 'mixed',      7.0, 'advanced')
)
-- source='benchmark': estes 12 são REFERÊNCIA nomeada; o motor passa a MONTAR WODs ('generated').
INSERT INTO wod (name, format_id, work_sec, rest_sec, rounds, emphasis_system, target_rpe, level, source)
SELECT w.name, f.id, w.work_sec, w.rest_sec, w.rounds, w.emphasis, w.target_rpe, w.level, 'benchmark'
FROM w JOIN wod_format f ON f.name = w.format;

-- FASE 5B (gerador): movimentos de GINÁSTICA icônicos que faltavam (muscle-ups e handstand walk). O
-- catálogo já tinha HSPU, Ring Dip, C2B, Pistol Squat, Handstand Hold, Toes-to-bar, Pull-up. focus=
-- 'conditioning' p/ ficarem na pista do WOD (não entram no funil de força e não mexem nos blocos atuais).
-- Bar Muscle-up e Ring Muscle-up JÁ existem no catálogo (focus strength) — só o Handstand Walk faltava.
WITH g(name, pattern, level, focus) AS (VALUES
  ('Handstand Walk', 'core', 'advanced', 'conditioning')
)
INSERT INTO exercise (name, movement_pattern_id, level, focus)
SELECT g.name, mp.id, g.level, g.focus FROM g JOIN movement_pattern mp ON mp.name = g.pattern;

-- FASE 5B (gerador): PERFIS de movimento (modalidade M/G/W, segundos-por-rep, skill). Placeholder
-- FUNDAMENTADO e calibrável (segundos-por-rep e skill são estimativas a calibrar com treinador). Cada
-- modalidade cobre do acessível (low) ao icônico (high); o nível + a skill travam combinações inseguras.
-- G = GINÁSTICA de verdade (pull-ups, toes-to-bar, dips, pistols, C2B, HSPU, muscle-ups, handstand walk).
-- Só MOVIMENTOS DE METCON entram (M/G/W). FICAM DE FORA, de propósito (são o trilho de força/técnica
-- que o motor já usa nas fases de acumulação/deload — não são movimentos de WOD):
--   técnica/drills puros: Tall/Drop/Muscle/Block Clean·Snatch, Snatch Balance, *High Pull, *Pull;
--   força pura: Back/Box/Pause/Tempo/Zercher Squat, Bench/Incline/DB Bench, Strict/Z/BTN Press, RDL,
--   Deficit/Snatch-grip DL, Good Morning, Hip Thrust, *Row de força, Weighted/Snatch-grip Pull-up;
--   holds/skill isolado: Plank, Hollow, L-sit, Handstand Hold, Pallof, Ab Wheel, Turkish Get-up.
WITH mp(movement, modality, secs_per_rep, skill) AS (VALUES
  -- M (monoestrutural): segundos por "rep/unidade" prescrita (cal, segmento curto, pulo de corda)
  ('Run',           'M', 3.5, 'low'),
  ('Row',           'M', 3.0, 'low'),
  ('Air Bike',      'M', 4.0, 'low'),
  ('Single Under',  'M', 0.4, 'low'),
  ('Double Under',  'M', 0.5, 'med'),
  -- G (ginástico): do acessível (peso do corpo) ao icônico (muscle-ups, handstands)
  ('Air Squat',           'G', 1.5, 'low'),
  ('Push-up',             'G', 1.5, 'low'),
  ('Burpee',              'G', 4.0, 'low'),
  ('Box Jump',            'G', 2.5, 'low'),
  ('Ring Row',            'G', 2.0, 'low'),
  ('Inverted Row',        'G', 2.0, 'low'),
  ('Walking Lunge',       'G', 2.0, 'low'),
  ('Step-up',             'G', 2.5, 'low'),
  ('Box Jump Over',       'G', 3.0, 'med'),
  ('Pull-up',             'G', 2.0, 'med'),
  ('Chin-up',             'G', 2.0, 'med'),
  ('Toes-to-bar',         'G', 2.5, 'med'),
  ('Ring Dip',            'G', 2.0, 'med'),
  ('Pike Push-up',        'G', 2.0, 'med'),
  ('Hanging Leg Raise',   'G', 2.5, 'med'),
  ('Chest-to-bar Pull-up','G', 2.5, 'high'),
  ('Handstand Push-up',   'G', 3.0, 'high'),
  ('Pistol Squat',        'G', 2.5, 'high'),
  ('Bar Muscle-up',       'G', 4.0, 'high'),
  ('Ring Muscle-up',      'G', 5.0, 'high'),
  ('Handstand Walk',      'G', 5.0, 'high'),
  -- W (halterofilismo/carga dinâmica de metcon): do acessível ao olímpico avançado
  ('Kettlebell Swing',  'W', 2.0, 'low'),
  ('Wall Ball',         'W', 2.5, 'low'),
  ('Kettlebell Clean',  'W', 2.5, 'low'),
  ('Goblet Squat',      'W', 2.5, 'low'),
  ('DB Shoulder Press', 'W', 2.0, 'low'),
  ('Deadlift',          'W', 3.0, 'low'),
  ('Sumo Deadlift',     'W', 3.0, 'low'),
  ('Front Squat',       'W', 3.0, 'low'),
  ('Push Press',        'W', 2.5, 'low'),
  ('Dumbbell Snatch',   'W', 2.5, 'med'),
  ('Push Jerk',         'W', 3.0, 'med'),
  ('Hang Power Clean',  'W', 3.0, 'med'),
  ('Hang Power Snatch', 'W', 3.0, 'med'),
  ('Power Clean',       'W', 3.5, 'med'),
  ('Thruster',          'W', 3.0, 'med'),
  ('Front Rack Lunge',  'W', 3.0, 'med'),
  ('Overhead Lunge',    'W', 3.5, 'med'),
  ('Power Snatch',      'W', 3.5, 'high'),
  ('Clean',             'W', 4.0, 'high'),
  ('Snatch',            'W', 4.0, 'high'),
  ('Hang Snatch',       'W', 3.5, 'high'),
  ('Clean and Jerk',    'W', 4.5, 'high'),
  ('Split Jerk',        'W', 3.5, 'high'),
  ('Overhead Squat',    'W', 3.0, 'high'),
  -- Carries (carga em deslocamento): secs_per_rep = segundos por unidade prescrita (ex.: ~10m)
  ('Farmer Carry',      'W', 2.5, 'low'),
  ('Suitcase Carry',    'W', 2.5, 'low'),
  ('Front Rack Carry',  'W', 3.0, 'low'),
  ('Overhead Carry',    'W', 3.5, 'med')
)
INSERT INTO movement_profile (exercise_id, modality, secs_per_rep, skill)
SELECT e.id, mp.modality, mp.secs_per_rep, mp.skill
FROM mp JOIN exercise e ON e.name = mp.movement;

-- Movimentos de cada WOD (reps NULL = "max"/contínuo, ex.: corrida por tempo). Por NOME.
WITH wm(wod, movement, reps, sort_order) AS (VALUES
  ('Sprint Intervals 10x15s', 'Run',          NULL, 1),
  ('Power EMOM Cleans',       'Power Clean',   2,   1),
  ('Box Jump Singles',        'Box Jump',      3,   1),
  ('90s Burpee Blast',        'Burpee',       NULL, 1),
  ('Thruster Sprint',         'Thruster',      21,  1),
  ('Row 500m',                'Row',          NULL, 1),
  ('Run 1km Steady',          'Run',          NULL, 1),
  ('Row 1000m',               'Row',          NULL, 1),
  ('Air Bike 6min',           'Air Bike',     NULL, 1),
  ('Bodyweight AMRAP 12',     'Pull-up',       5,   1),
  ('Bodyweight AMRAP 12',     'Push-up',       10,  2),
  ('Bodyweight AMRAP 12',     'Air Squat',     15,  3),
  ('Chipper 15',              'Wall Ball',     50,  1),
  ('Chipper 15',              'Box Jump',      40,  2),
  ('Chipper 15',              'Kettlebell Swing', 30, 3),
  ('Chipper 15',              'Burpee',        20,  4),
  ('Engine AMRAP 20',         'Row',          NULL, 1),
  ('Engine AMRAP 20',         'Run',          NULL, 2),
  ('Engine AMRAP 20',         'Burpee',        10,  3)
)
INSERT INTO wod_movement (wod_id, exercise_id, reps, sort_order)
SELECT wd.id, e.id, wm.reps, wm.sort_order
FROM wm
  JOIN wod wd      ON wd.name = wm.wod
  JOIN exercise e  ON e.name = wm.movement;
