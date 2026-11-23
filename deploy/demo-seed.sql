-- Supply owner_email of an existing LOCAL demo account using psql -v.
\set ON_ERROR_STOP on
BEGIN;
SELECT id AS owner FROM users WHERE email=lower(:'owner_email') \gset
INSERT INTO links(user_id,domain_id,name,slug,target_url,target_host)
 SELECT :'owner',id,'Демонстрационная документация','demo-'||substr(:'owner',1,8),'https://example.org/docs','example.org'
 FROM domains WHERE kind='system' ON CONFLICT(domain_id,slug) DO NOTHING;
INSERT INTO monitors(user_id,link_id,name,url,kind,method,interval_seconds,timeout_ms,status_min,status_max,enabled,status)
 SELECT :'owner',id,'Демонстрация истории','https://example.org/docs','link_target','HEAD',300,5000,200,299,false,'paused'
 FROM links WHERE user_id=:'owner' AND slug='demo-'||substr(:'owner',1,8)
 AND name='Демонстрационная документация' AND target_url='https://example.org/docs'
 AND domain_id=(SELECT id FROM domains WHERE kind='system') ON CONFLICT(link_id) DO NOTHING;
INSERT INTO monitor_checks(monitor_id,scheduled_at,checked_at,http_status,latency_ms,success,error_type)
 SELECT m.id,m.created_at-i*interval '5 minutes',m.created_at-i*interval '5 minutes',CASE WHEN i IN (3,4) THEN 503 ELSE 200 END,
 90+i*11,i NOT IN (3,4),CASE WHEN i IN (3,4) THEN 'http_status' END
 FROM monitors m JOIN links l ON m.link_id=l.id CROSS JOIN generate_series(1,8)i
 WHERE l.user_id=:'owner' AND l.slug='demo-'||substr(:'owner',1,8)
 AND l.domain_id=(SELECT id FROM domains WHERE kind='system') AND NOT m.enabled
 AND m.name='Демонстрация истории' AND m.url='https://example.org/docs'
 ON CONFLICT(monitor_id,scheduled_at) DO NOTHING;
COMMIT;
