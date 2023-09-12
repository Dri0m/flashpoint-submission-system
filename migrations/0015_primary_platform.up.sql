ALTER TABLE curation_meta ADD primary_platform TEXT;
UPDATE curation_meta
SET primary_platform = SUBSTRING_INDEX(platform, ';', 1);