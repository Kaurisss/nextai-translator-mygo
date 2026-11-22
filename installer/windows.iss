#ifndef AppVersion
  #define AppVersion "0.4.0"
#endif
#ifndef SourceDir
  #define SourceDir "..\build\settings\windows-amd64"
#endif

[Setup]
AppId=xyz.nextai.translator.native
AppName=NextAI Translator Native
AppVersion={#AppVersion}
AppPublisher=NextAI Translator
DefaultDirName={localappdata}\Programs\NextAI Translator Native
DefaultGroupName=NextAI Translator Native
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir=..\build\installer
OutputBaseFilename=nextai-translator-native-setup-{#AppVersion}-x64
SetupIconFile=..\resources\icon.ico
UninstallDisplayIcon={app}\NextAI Translator Native.exe
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
CloseApplications=yes
RestartApplications=no

[Languages]
Name: "chinesesimplified"; MessagesFile: "compiler:Languages\ChineseSimplified.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "{#SourceDir}\NextAI Translator Native.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\icon.png"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\LICENSE-icons.txt"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\NextAI Translator Native"; Filename: "{app}\NextAI Translator Native.exe"
Name: "{autodesktop}\NextAI Translator Native"; Filename: "{app}\NextAI Translator Native.exe"; Tasks: desktopicon

[Run]
Filename: "{app}\NextAI Translator Native.exe"; Description: "{cm:LaunchProgram,NextAI Translator Native}"; Flags: nowait postinstall skipifsilent
