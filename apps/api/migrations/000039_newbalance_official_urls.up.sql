UPDATE products AS p
SET product_url = v.product_url
FROM (VALUES
    ('New Balance', 'FuelCell SuperComp Elite v5', 'https://outlet.newbalance.jp/pd/MRCELV5-50683-PMG-APAC.html'),
    ('New Balance', 'FuelCell SuperComp MD-X v3', 'https://shop.newbalance.jp/pd/UMDELRSV3-51853.html'),
    ('New Balance', 'FuelCell SuperComp LD-X v3', 'https://shop.newbalance.jp/pd/UELRLDV3_RU-FTW-820504.html')
) AS v (brand, name, product_url)
WHERE lower(p.brand) = lower(v.brand)
  AND lower(p.name) = lower(v.name);
