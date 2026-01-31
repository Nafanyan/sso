INSERT INTO apps (id, code, secret)
VALUES 
    (1, 'test', 'test-secret'),
    (2, 'web', 'web-secret-key'),
    (3, 'mobile', 'mobile-secret-key'),
    (4, 'api', 'api-secret-key'),
    (5, 'admin', 'admin-secret-key')
ON CONFLICT DO NOTHING;