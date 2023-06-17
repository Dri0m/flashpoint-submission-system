ALTER TABLE tag
ADD CONSTRAINT "FK_tag_primary_alias" FOREIGN KEY ("primary_alias")
REFERENCES "tag_alias"("name")
ON DELETE CASCADE
ON UPDATE NO ACTION;