UPDATE products AS p
SET product_url = ''
FROM (VALUES
    ('New Balance', 'SuperComp Elite v6'),
    ('HOKA', 'Cielo X1 3.0'),
    ('HOKA', 'Rocket X 3'),
    ('HOKA', 'Cielo X 2 MD'),
    ('HOKA', 'Cielo FlyX Lite'),
    ('Mizuno', 'CHRONO INX NEO JAPAN'),
    ('adidas', 'Adizero Prime SP 3 Strung')
) AS v (brand, name)
WHERE lower(p.brand) = lower(v.brand)
  AND lower(p.name) = lower(v.name);
