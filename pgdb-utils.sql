-- Disable and enable triggers
SET session_replication_role = replica;
SET session_replication_role = DEFAULT;

-- Rebuild platform strings

UPDATE game
SET platforms_str = (
    SELECT string_agg(
                   (SELECT primary_alias FROM platform WHERE id = p.platform_id), '; '
               )
    FROM game_platforms_platform p
    WHERE p.game_id = game.id
) WHERE 1=1

-- Rebuild tag strings

UPDATE game
SET tags_str = coalesce(
    (
        SELECT string_agg(
                       (SELECT primary_alias FROM tag WHERE id = t.tag_id), '; '
                   )
        FROM game_tags_tag t
        WHERE t.game_id = game.id
    ), ''
) WHERE 1=1