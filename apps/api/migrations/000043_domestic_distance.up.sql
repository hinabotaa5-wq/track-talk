ALTER TABLE domestic_majors DROP CONSTRAINT domestic_majors_category_check;
ALTER TABLE domestic_majors
    ADD CONSTRAINT domestic_majors_category_check CHECK (category IN ('youth', 'student', 'open', 'distance'));
