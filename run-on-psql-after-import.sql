BEGIN;
SELECT SETVAL('public.changelog_platform_id_seq', COALESCE(MAX(id), 1) ) FROM public.changelog_platform;
SELECT SETVAL('public.changelog_tag_id_seq', COALESCE(MAX(id), 1) ) FROM public.changelog_tag;
SELECT SETVAL('public.game_data_id_seq', COALESCE(MAX(id), 1) ) FROM public.game_data;
SELECT SETVAL('public.platform_id_seq', COALESCE(MAX(id), 1) ) FROM public.platform;
SELECT SETVAL('public.tag_category_id_seq', COALESCE(MAX(id), 1) ) FROM public.tag_category;
SELECT SETVAL('public.tag_id_seq', COALESCE(MAX(id), 1) ) FROM public.tag;
COMMIT;