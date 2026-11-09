# Seeds

Dados iniciais e demonstrativos devem ser mantidos separados das migrations estruturais.

`development.sql` contém um catálogo fictício para desenvolvimento local. Ele pressupõe que todas as migrations foram aplicadas e pode ser executado com:

```powershell
Get-Content -Raw seeds/development.sql | psql $env:DATABASE_URL
```

Os UUIDs são determinísticos e os registros usam `ON CONFLICT`, permitindo reaplicação no banco de desenvolvimento. O seed não deve ser usado como catálogo de produção.

