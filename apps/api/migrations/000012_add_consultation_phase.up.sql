ALTER TABLE consultation_sessions
ADD COLUMN phase VARCHAR(30) NOT NULL DEFAULT 'collecting';

CREATE INDEX idx_consultation_sessions_phase ON consultation_sessions(phase);
