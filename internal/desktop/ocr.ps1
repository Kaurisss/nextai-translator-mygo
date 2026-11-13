$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)
Add-Type -AssemblyName System.Runtime.WindowsRuntime
$null = [Windows.Storage.StorageFile, Windows.Storage, ContentType=WindowsRuntime]
$null = [Windows.Graphics.Imaging.BitmapDecoder, Windows.Graphics.Imaging, ContentType=WindowsRuntime]
$null = [Windows.Media.Ocr.OcrEngine, Windows.Foundation, ContentType=WindowsRuntime]
$asTask = [System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object { $_.Name -eq 'AsTask' -and $_.IsGenericMethod -and $_.GetParameters().Count -eq 1 -and $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation`1' } | Select-Object -First 1
function Await($operation, $type) {
    $task = $asTask.MakeGenericMethod($type).Invoke($null, @($operation))
    $task.GetAwaiter().GetResult()
}
$file = Await ([Windows.Storage.StorageFile]::GetFileFromPathAsync($env:NEXTAI_OCR_PATH)) ([Windows.Storage.StorageFile])
$stream = Await ($file.OpenAsync([Windows.Storage.FileAccessMode]::Read)) ([Windows.Storage.Streams.IRandomAccessStream])
try {
    $decoder = Await ([Windows.Graphics.Imaging.BitmapDecoder]::CreateAsync($stream)) ([Windows.Graphics.Imaging.BitmapDecoder])
    $bitmap = Await ($decoder.GetSoftwareBitmapAsync()) ([Windows.Graphics.Imaging.SoftwareBitmap])
    try {
        $engines = [Collections.Generic.List[object]]::new()
        $profile = [Windows.Media.Ocr.OcrEngine]::TryCreateFromUserProfileLanguages()
        if ($null -ne $profile) { $engines.Add($profile) }
        foreach ($language in [Windows.Media.Ocr.OcrEngine]::AvailableRecognizerLanguages) {
            if ($null -ne $profile -and $language.LanguageTag -eq $profile.RecognizerLanguage.LanguageTag) { continue }
            $engine = [Windows.Media.Ocr.OcrEngine]::TryCreateFromLanguage($language)
            if ($null -ne $engine) { $engines.Add($engine) }
        }
        if ($engines.Count -eq 0) { throw 'Install an OCR language pack in Windows Settings.' }
        $results = [Collections.Generic.List[string]]::new()
        $failures = [Collections.Generic.List[string]]::new()
        foreach ($engine in $engines) {
            try {
                $result = Await ($engine.RecognizeAsync($bitmap)) ([Windows.Media.Ocr.OcrResult])
                $results.Add(($result.Lines | ForEach-Object { $_.Text.Trim() }) -join "`n")
            } catch {
                $failures.Add($_.Exception.Message)
            }
        }
        if ($results.Count -eq 0) { throw ($failures -join "`n") }
        ConvertTo-Json -InputObject @($results.ToArray()) -Compress
    } finally { $bitmap.Dispose() }
} finally { $stream.Dispose() }
