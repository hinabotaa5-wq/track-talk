UPDATE products AS p
SET product_url = ''
FROM (VALUES
    ('New Balance', 'FuelCell SuperComp Elite v5'),
    ('New Balance', 'FuelCell SuperComp MD-X v3'),
    ('New Balance', 'FuelCell SuperComp LD-X v3')
) AS v (brand, name)
WHERE lower(p.brand) = lower(v.brand)
  AND lower(p.name) = lower(v.name);
