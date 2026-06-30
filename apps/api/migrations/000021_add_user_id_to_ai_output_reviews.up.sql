-- Add user_id column to ai_output_reviews (required by 05b plan)
ALTER TABLE ai_output_reviews
    ADD COLUMN IF NOT EXISTS user_id UUID;

CREATE INDEX IF NOT EXISTS idx_ai_output_reviews_user_id ON ai_output_reviews(user_id);
