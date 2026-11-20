[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'

$expectedHost = @('localhost', '127.0.0.1', '::1')
$expectedPort = 5433
$expectedDatabase = 'loja_3d_test'
$expectedUser = 'formo3d_test'
$expectedContainer = 'formo3d-postgres-test'
$expectedVolume = 'formo3d_postgres_test_data'
$expectedImage = 'postgres:17.6-alpine'
$expectedProject = 'formo3d-test'
$expectedService = 'postgres-test'
$expectedVersion = 28
$secretInContainer = '/run/secrets/test_postgres_password'

$backendDir = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$repoRoot = [IO.Path]::GetFullPath((Join-Path $backendDir '..'))
$sourceDir = Join-Path $backendDir 'migrations'
$originalLocation = Get-Location
$originalMigrationsPath = [Environment]::GetEnvironmentVariable('MIGRATIONS_PATH', 'Process')
$databaseUrlBefore = [Environment]::GetEnvironmentVariable('DATABASE_URL', 'Process')
$temporaryDirectory = $null
$completed = $false

function Stop-Safely([string]$Message) {
    throw $Message
}

function Invoke-TestPsql([string]$Sql) {
    $shell = 'export PGPASSWORD="$(cat ' + $secretInContainer + ')"; exec psql -h 127.0.0.1 -p 5432 -U ' + $expectedUser + ' -d ' + $expectedDatabase + ' -X -v ON_ERROR_STOP=1 -At -F "|" -c "$1"'
    $output = & docker exec $expectedContainer sh -c $shell sh $Sql 2>$null
    if ($LASTEXITCODE -ne 0) {
        Stop-Safely 'Consulta read-only no PostgreSQL de teste falhou; saída sensível foi omitida.'
    }
    return (($output | Out-String).Trim())
}

function Invoke-Migrator([string]$Operation) {
    Push-Location $backendDir
    try {
        $output = & go run ./cmd/migrate $Operation 2>&1
        $exitCode = $LASTEXITCODE
    }
    finally {
        Pop-Location
    }
    if ($exitCode -ne 0) {
        # Do not echo raw migrator output: errors may contain connection details.
        Stop-Safely "Migrador '$Operation' falhou (exit code $exitCode); saída omitida para proteger credenciais."
    }
    return (($output | Out-String).Trim())
}

try {
    if ([string]::IsNullOrWhiteSpace($databaseUrlBefore)) {
        Stop-Safely 'DATABASE_URL ausente neste PowerShell; nenhuma migration foi executada.'
    }

    try {
        $databaseUri = [uri]$databaseUrlBefore
    }
    catch {
        Stop-Safely 'DATABASE_URL inválida; nenhuma migration foi executada.'
    }

    $databaseUser = [uri]::UnescapeDataString(($databaseUri.UserInfo -split ':', 2)[0])
    $databaseName = [uri]::UnescapeDataString($databaseUri.AbsolutePath.TrimStart('/'))
    if ($databaseUri.Host -notin $expectedHost -or
        $databaseUri.Port -ne $expectedPort -or
        $databaseName -cne $expectedDatabase -or
        $databaseUser -cne $expectedUser) {
        Stop-Safely 'DATABASE_URL não corresponde ao destino de teste esperado; URL e credenciais não foram exibidas.'
    }
    Write-Host 'Guard DATABASE_URL aprovado: loopback:5433/loja_3d_test, usuário de teste.'

    $containerState = & docker inspect --format '{{.State.Status}}|{{if .State.Health}}{{.State.Health.Status}}{{else}}sem-healthcheck{{end}}' $expectedContainer 2>$null
    if ($LASTEXITCODE -ne 0 -or $containerState -cne 'running|healthy') {
        Stop-Safely 'Container de teste não está running/healthy; nenhuma migration foi executada.'
    }

    $image = & docker inspect --format '{{.Config.Image}}' $expectedContainer 2>$null
    if ($LASTEXITCODE -ne 0 -or $image -cne $expectedImage) {
        Stop-Safely 'Imagem/versão configurada do container não corresponde a PostgreSQL 17.6 Alpine.'
    }

    $labelsRaw = & docker inspect --format '{{json .Config.Labels}}' $expectedContainer 2>$null
    if ($LASTEXITCODE -ne 0) {
        Stop-Safely 'Não foi possível verificar as labels Compose do container.'
    }
    $labels = $labelsRaw | ConvertFrom-Json
    if ($labels.'com.docker.compose.project' -cne $expectedProject -or
        $labels.'com.docker.compose.service' -cne $expectedService) {
        Stop-Safely 'Container não pertence ao projeto/serviço de testes esperado.'
    }

    $mountsRaw = & docker inspect --format '{{json .Mounts}}' $expectedContainer 2>$null
    if ($LASTEXITCODE -ne 0) {
        Stop-Safely 'Não foi possível verificar os mounts do container.'
    }
    $mounts = @($mountsRaw | ConvertFrom-Json)
    $dataMounts = @($mounts | Where-Object { $_.Type -ceq 'volume' })
    $secretMounts = @($mounts | Where-Object { $_.Type -ceq 'bind' })
    $unexpectedMounts = @($mounts | Where-Object { $_.Type -cnotin @('volume', 'bind') })
    if ($mounts.Count -ne 2 -or
        $dataMounts.Count -ne 1 -or
        $secretMounts.Count -ne 1 -or
        $unexpectedMounts.Count -ne 0 -or
        $dataMounts[0].Name -cne $expectedVolume -or
        $dataMounts[0].Destination -cne '/var/lib/postgresql/data' -or
        $dataMounts[0].RW -ne $true -or
        $dataMounts[0].Name -ceq 'postgres_data' -or
        [string]::IsNullOrWhiteSpace($secretMounts[0].Source) -or
        $secretMounts[0].Destination -cne $secretInContainer -or
        $secretMounts[0].RW -ne $false) {
        Stop-Safely 'Mounts inesperados: esperado somente o volume de dados de teste e o bind read-only do secret.'
    }

    $portsRaw = & docker inspect --format '{{json .NetworkSettings.Ports}}' $expectedContainer 2>$null
    if ($LASTEXITCODE -ne 0) {
        Stop-Safely 'Não foi possível verificar a publicação de portas do container.'
    }
    $ports = $portsRaw | ConvertFrom-Json
    $published = @($ports.'5432/tcp')
    if ($published.Count -ne 1 -or
        $published[0].HostIp -cne '127.0.0.1' -or
        $published[0].HostPort -cne '5433') {
        Stop-Safely 'A publicação não é exclusivamente 127.0.0.1:5433 -> 5432.'
    }
    Write-Host 'Guard Docker aprovado: container saudável, projeto e volume de dados exclusivos, secret bind read-only e bind loopback.'

    $identity = Invoke-TestPsql "SELECT current_database(), current_user, current_setting('server_version');"
    $identityParts = $identity.Split('|')
    if ($identityParts.Count -ne 3 -or
        $identityParts[0] -cne $expectedDatabase -or
        $identityParts[1] -cne $expectedUser -or
        $identityParts[2] -cne '17.6') {
        Stop-Safely 'Identidade PostgreSQL inesperada; nenhuma migration foi executada.'
    }

    $migrationTableState = Invoke-TestPsql "SELECT CASE WHEN to_regclass('public.schema_migrations') IS NULL THEN 'absent' ELSE 'present' END;"
    if ($migrationTableState -ceq 'absent') {
        $initialVersion = 'none'
    }
    elseif ($migrationTableState -ceq 'present') {
        $migrationRowCount = Invoke-TestPsql 'SELECT count(*) FROM public.schema_migrations;'
        if ($migrationRowCount -cne '0') {
            Stop-Safely 'O estado inicial do migrador não é version none; nenhuma migration foi executada.'
        }
        $initialVersion = 'none'
    }
    else {
        Stop-Safely 'Não foi possível determinar de forma read-only o estado inicial das migrations.'
    }
    if ($initialVersion -cne 'none') {
        Stop-Safely 'O estado inicial não é version none; nenhuma migration foi executada.'
    }
    Write-Host 'Estado inicial verificado por SELECT read-only: version none.'

    $migrationFiles = @(Get-ChildItem -LiteralPath $sourceDir -File)
    $upFiles = @{}
    $downFiles = @{}
    foreach ($file in $migrationFiles) {
        if ($file.Name -match '^(\d{6})_[A-Za-z0-9_-]+\.(up|down)\.sql$') {
            $version = [int]$Matches[1]
            $direction = $Matches[2]
            if ($version -gt $expectedVersion) {
                Stop-Safely 'Foi encontrada migration posterior à 28.'
            }
            if ($direction -eq 'up') {
                if ($upFiles.ContainsKey($version)) { Stop-Safely 'Migration UP duplicada.' }
                $upFiles[$version] = $file
            }
            else {
                if ($downFiles.ContainsKey($version)) { Stop-Safely 'Migration DOWN duplicada.' }
                $downFiles[$version] = $file
            }
        }
        elseif ($file.Name -match '^\d+') {
            Stop-Safely 'Arquivo numerado com formato de migration inesperado.'
        }
    }

    if ($upFiles.Count -ne $expectedVersion -or $downFiles.Count -ne $expectedVersion) {
        Stop-Safely 'A origem não contém exatamente 28 arquivos UP e 28 DOWN.'
    }
    for ($version = 1; $version -le $expectedVersion; $version++) {
        if (-not $upFiles.ContainsKey($version) -or -not $downFiles.ContainsKey($version)) {
            Stop-Safely "Par UP/DOWN ausente na versão $version."
        }
    }
    Write-Host 'Guard de migrations aprovado: exatamente 28 pares UP/DOWN, sem versões posteriores.'

    if ([string]::IsNullOrWhiteSpace($env:TEMP)) {
        Stop-Safely 'TEMP não está definido; não é seguro criar cópia temporária.'
    }
    $tempRoot = [IO.Path]::GetFullPath($env:TEMP).TrimEnd('\') + '\'
    $candidate = Join-Path $tempRoot ('formo3d-migrations-test-' + [guid]::NewGuid().ToString('N'))
    $candidateFull = [IO.Path]::GetFullPath($candidate)
    if (-not $candidateFull.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase) -or
        $candidateFull.StartsWith($repoRoot, [StringComparison]::OrdinalIgnoreCase)) {
        Stop-Safely 'Diretório temporário não está seguramente fora do repositório.'
    }
    New-Item -ItemType Directory -Path $candidateFull -ErrorAction Stop | Out-Null
    $temporaryDirectory = $candidateFull

    foreach ($version in 1..$expectedVersion) {
        Copy-Item -LiteralPath $upFiles[$version].FullName -Destination $temporaryDirectory
        Copy-Item -LiteralPath $downFiles[$version].FullName -Destination $temporaryDirectory
    }

    $copiedFiles = @(Get-ChildItem -LiteralPath $temporaryDirectory -File)
    if ($copiedFiles.Count -ne 56) {
        Stop-Safely 'A cópia temporária não contém exatamente 56 arquivos.'
    }
    foreach ($original in @($upFiles.Values) + @($downFiles.Values)) {
        $copy = Join-Path $temporaryDirectory $original.Name
        if (-not (Test-Path -LiteralPath $copy -PathType Leaf)) {
            Stop-Safely "Arquivo ausente na cópia temporária: $($original.Name)."
        }
        $sourceHash = (Get-FileHash -LiteralPath $original.FullName -Algorithm SHA256).Hash
        $copyHash = (Get-FileHash -LiteralPath $copy -Algorithm SHA256).Hash
        if ($sourceHash -cne $copyHash) {
            Stop-Safely "Hash SHA-256 divergente: $($original.Name)."
        }
    }
    $copiedMigrationFiles = @(Get-ChildItem -LiteralPath $temporaryDirectory -File | Where-Object { $_.Name -match '^(\d{6})_.*\.(up|down)\.sql$' })
    if ($copiedMigrationFiles.Count -ne 56 -or
        @($copiedMigrationFiles | Where-Object { [int]($_.Name.Substring(0, 6)) -gt $expectedVersion }).Count -gt 0) {
        Stop-Safely 'Conteúdo da cópia temporária excede ou diverge das versões 1–28.'
    }
    Write-Host "Cópia temporária validada: $temporaryDirectory (56 arquivos; todos os SHA-256 conferem)."

    # golang-migrate's Windows file source concatenates URL host + path;
    # file://C:/... therefore resolves to C:/... in v4.20.1.
    if ($temporaryDirectory -notmatch '^[A-Za-z]:\\') {
        Stop-Safely 'TEMP não está em unidade local Windows; não foi possível formar source file:// seguro.'
    }
    $migrationSource = 'file://' + $temporaryDirectory.Replace('\', '/')
    [Environment]::SetEnvironmentVariable('MIGRATIONS_PATH', $migrationSource, 'Process')

    if ([Environment]::GetEnvironmentVariable('DATABASE_URL', 'Process') -cne $databaseUrlBefore) {
        Stop-Safely 'DATABASE_URL mudou durante as validações; UP não foi executado.'
    }

    Write-Host 'Todas as proteções passaram; iniciando UP apenas com a cópia temporária e o banco de testes.'
    $null = Invoke-Migrator 'up'

    $finalState = Invoke-Migrator 'version'
    if ($finalState -cnotmatch '^version:\s*28,\s*dirty:\s*false$') {
        Stop-Safely 'Estado final diferente de version 28, dirty false; cópia preservada para diagnóstico.'
    }

    $finalIdentity = Invoke-TestPsql "SELECT current_database(), current_user, current_setting('server_version');"
    $finalParts = $finalIdentity.Split('|')
    if ($finalParts.Count -ne 3 -or
        $finalParts[0] -cne $expectedDatabase -or
        $finalParts[1] -cne $expectedUser -or
        $finalParts[2] -cne '17.6') {
        Stop-Safely 'Identidade final do banco divergiu do destino de teste.'
    }

    if ([Environment]::GetEnvironmentVariable('DATABASE_URL', 'Process') -cne $databaseUrlBefore) {
        Stop-Safely 'DATABASE_URL mudou durante a aplicação; cópia preservada.'
    }

    $resolvedTemp = [IO.Path]::GetFullPath($temporaryDirectory)
    $parent = [IO.Path]::GetFullPath((Split-Path -Parent $resolvedTemp)).TrimEnd('\') + '\'
    $leaf = Split-Path -Leaf $resolvedTemp
    if ($parent -cne $tempRoot -or $leaf -notmatch '^formo3d-migrations-test-[0-9a-f]{32}$') {
        Stop-Safely 'A cópia temporária não passou pela validação de alvo para limpeza.'
    }
    Remove-Item -LiteralPath $resolvedTemp -Recurse -Force -ErrorAction Stop
    $temporaryDirectory = $null
    $completed = $true

    Write-Host 'Resultado: PostgreSQL 17.6; loja_3d_test/formo3d_test; migration 28; dirty=false.'
    Write-Host 'Cópia temporária removida após validação. Originais, container e volume preservados.'
}
catch {
    if ($temporaryDirectory) {
        Write-Error ("Operação interrompida. Cópia temporária preservada para diagnóstico em: " + $temporaryDirectory)
    }
    throw
}
finally {
    [Environment]::SetEnvironmentVariable('MIGRATIONS_PATH', $originalMigrationsPath, 'Process')
    Set-Location $originalLocation
}

if (-not $completed) {
    Stop-Safely 'A aplicação não foi concluída.'
}
