UPDATE products AS p
SET product_url = v.product_url
FROM (VALUES
    ('ASICS', 'METASPEED SKY TOKYO', 'https://www.asics.com/gb/en-gb/metaspeed-sky-tokyo/p/1013A162-400.html'),
    ('ASICS', 'METASPEED EDGE TOKYO', 'https://www.asics.com/gb/en-gb/metaspeed-edge-tokyo/p/1013A163-400.html'),
    ('ASICS', 'METASPEED SP 2', 'https://www.asics.com/be/en-be/metaspeed-sp-2/p/1093A234-101.html'),
    ('ASICS', 'METASPEED LD 2', 'https://www.asics.com/be/en-be/metaspeed-ld-2/p/1093A251-101.html'),
    ('Nike', 'Alphafly 4', 'https://www.nike.com/t/alphafly-4-mens-road-racing-shoes-lw5UiD4C'),
    ('Nike', 'Vaporfly 4', 'https://www.nike.com/t/vaporfly-4-mens-road-racing-shoes-HK05JWOf/IR0078-100'),
    ('Nike', 'Maxfly 2', 'https://www.nike.com/t/maxfly-2-track-field-sprinting-spikes-fBfStC/FD8395-601'),
    ('Nike', 'Dragonfly 2', 'https://www.nike.com/t/dragonfly-2-track-field-distance-spikes-03rvtN/FD8413-101'),
    ('adidas', 'Adizero Adios Pro 4', 'https://www.adidas.com/us/adizero_adios_pro_4'),
    ('Mizuno', 'WAVE REBELLION PRO 3', 'https://mys.mizuno.com/products/wave-rebellion-pro-3-u1gd2530-2025ss'),
    ('Mizuno', 'WAVE REBELLION FLASH 3', 'https://www.mizuno.com.au/products/wave-rebellion-flash-3-hanabi'),
    ('PUMA', 'Fast-R NITRO Elite 3', 'https://us.puma.com/us/en/pd/fast-r-nitro-elite-3-mens-road-running-shoes/312060'),
    ('PUMA', 'Deviate NITRO Elite 3', 'https://us.puma.com/us/en/pd/deviate-nitro-elite-3-mens-running-shoes/309809'),
    ('PUMA', 'evoSPEED Long Distance NITRO Elite+', 'https://us.puma.com/us/en/pd/evospeed-long-distance-nitro-elite-track-spikes/378222'),
    ('On', 'Cloudboom Strike LS', 'https://www.on.com/en-us/products/cloudboom-strike-ls-u-3ue3001'),
    ('On', 'Cloudboom Strike 2', 'https://www.on.com/en-us/products/cloudboom-strike-2-u-3ug3005'),
    ('On', 'Cloudspike Citius 2', 'https://www.on.com/en-us/products/cloudspike-citius-2-m-3mf1030/mens')
) AS v (brand, name, product_url)
WHERE lower(p.brand) = lower(v.brand)
  AND lower(p.name) = lower(v.name);
