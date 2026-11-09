DROP INDEX IF EXISTS coaching_plans_consult_template_idx;
ALTER TABLE coaching_plans DROP COLUMN IF EXISTS consult_template;
DROP TABLE IF EXISTS support_consult_proposals;
DROP TABLE IF EXISTS support_consult_questions;
DROP TABLE IF EXISTS support_consults;
DROP TABLE IF EXISTS support_consult_settings;
