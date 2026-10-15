UPDATE products AS p
SET product_url = v.product_url
FROM (VALUES
    ('New Balance', 'SuperComp Elite v6', 'https://shop.newbalance.jp/lp-running-supercomp.html'),
    ('HOKA', 'Cielo X1 3.0', 'https://www.hoka.com/en/us/all-gender-footwear/cielo-x1-3.0/1171927.html'),
    ('HOKA', 'Rocket X 3', 'https://www.hoka.com/en/us/race-shoes/rocket-x-3/1168724.html'),
    ('HOKA', 'Cielo X 2 MD', 'https://www.hoka.com/en/us/all-gender-footwear/cielo-x-2-md/1134534.html'),
    ('HOKA', 'Cielo FlyX Lite', 'https://www.hoka.com/en/us/all-gender-footwear/cielo-flyx/1151990.html'),
    ('Mizuno', 'CHRONO INX NEO JAPAN', 'https://jpn.mizuno.com/ec/disp/attgrp/U1GA2400/'),
    ('adidas', 'Adizero Prime SP 3 Strung', 'https://www.adidas.jp/%E3%82%A2%E3%83%87%E3%82%A3%E3%82%BC%E3%83%AD-%E3%83%97%E3%83%A9%E3%82%A4%E3%83%A0-sp-3-strung-adizero-prime-sp-3-strung/IH5801.html')
) AS v (brand, name, product_url)
WHERE lower(p.brand) = lower(v.brand)
  AND lower(p.name) = lower(v.name);
