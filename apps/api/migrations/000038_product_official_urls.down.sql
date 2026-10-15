UPDATE products AS p
SET product_url = ''
FROM (VALUES
    ('ASICS', 'METASPEED SKY TOKYO'),
    ('ASICS', 'METASPEED EDGE TOKYO'),
    ('ASICS', 'METASPEED SP 2'),
    ('ASICS', 'METASPEED LD 2'),
    ('Nike', 'Alphafly 4'),
    ('Nike', 'Vaporfly 4'),
    ('Nike', 'Maxfly 2'),
    ('Nike', 'Dragonfly 2'),
    ('adidas', 'Adizero Adios Pro 4'),
    ('Mizuno', 'WAVE REBELLION PRO 3'),
    ('Mizuno', 'WAVE REBELLION FLASH 3'),
    ('PUMA', 'Fast-R NITRO Elite 3'),
    ('PUMA', 'Deviate NITRO Elite 3'),
    ('PUMA', 'evoSPEED Long Distance NITRO Elite+'),
    ('On', 'Cloudboom Strike LS'),
    ('On', 'Cloudboom Strike 2'),
    ('On', 'Cloudspike Citius 2')
) AS v (brand, name)
WHERE lower(p.brand) = lower(v.brand)
  AND lower(p.name) = lower(v.name);
