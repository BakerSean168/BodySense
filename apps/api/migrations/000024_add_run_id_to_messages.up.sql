ALTER TABLE messages ADD COLUMN run_id UUID REFERENCES runs(id);
CREATE INDEX idx_messages_run_id ON messages(run_id);
