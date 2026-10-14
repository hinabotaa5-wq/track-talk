UPDATE products AS p
SET image_url = v.image_url
FROM (VALUES
    ('ASICS', 'METASPEED SKY TOKYO', '/products/asics-metaspeed-sky-tokyo.png'),
    ('ASICS', 'METASPEED EDGE TOKYO', '/products/asics-metaspeed-edge-tokyo.png'),
    ('ASICS', 'METASPEED SP 2', '/products/asics-metaspeed-sp-2.png'),
    ('ASICS', 'METASPEED LD 2', '/products/asics-metaspeed-ld-2.png'),
    ('Nike', 'Alphafly 4', '/products/nike-alphafly-4.png'),
    ('Nike', 'Vaporfly 4', '/products/nike-vaporfly-4.png'),
    ('Nike', 'Maxfly 2', '/products/nike-maxfly-2.png'),
    ('Nike', 'Dragonfly 2', '/products/nike-dragonfly-2.png'),
    ('adidas', 'Adizero Adios Pro 4', '/products/adidas-adizero-adios-pro-4.jpg'),
    ('Mizuno', 'WAVE REBELLION PRO 3', '/products/mizuno-wave-rebellion-pro-3.jpg'),
    ('Mizuno', 'WAVE REBELLION FLASH 3', '/products/mizuno-wave-rebellion-flash-3.jpg'),
    ('Mizuno', 'CHRONO INX NEO JAPAN', '/products/mizuno-chrono-inx-neo-japan.png'),
    ('New Balance', 'SuperComp Elite v6', '/products/nb-supercomp-elite-v6.png'),
    ('New Balance', 'FuelCell SuperComp MD-X v3', '/products/nb-supercomp-md-x-v3.png'),
    ('New Balance', 'FuelCell SuperComp LD-X v3', '/products/nb-supercomp-ld-x-v3.png'),
    ('PUMA', 'Fast-R NITRO Elite 3', '/products/puma-fast-r-nitro-elite-3.jpg'),
    ('PUMA', 'Deviate NITRO Elite 3', '/products/puma-deviate-nitro-elite-3.jpg'),
    ('PUMA', 'evoSPEED Long Distance NITRO Elite+', '/products/puma-evospeed-long-distance-nitro-elite-plus.jpg'),
    ('On', 'Cloudspike Citius 2', '/products/on-cloudspike-citius-2.png'),
    ('On', 'Cloudboom Strike LS', '/products/on-cloudboom-strike-ls.png'),
    ('On', 'Cloudboom Strike 2', '/products/on-cloudboom-strike-2.png')
) AS v (brand, name, image_url)
WHERE lower(p.brand) = lower(v.brand)
  AND lower(p.name) = lower(v.name);
