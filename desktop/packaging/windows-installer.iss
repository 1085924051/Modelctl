#define MyAppName "Modelctl"
#define MyAppVersion "0.1.0"
#define MyAppPublisher "Modelctl"
#define MyAppExeName "Modelctl.exe"

[Setup]
AppId={{A3C5C2AF-2F02-4CF5-AFF1-MODELCTL0001}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={autopf}\Modelctl
DefaultGroupName=Modelctl
OutputBaseFilename=Modelctl-Setup-v{#MyAppVersion}
ArchitecturesInstallIn64BitMode=x64
Compression=lzma2
SolidCompression=yes
Uninstallable=yes

[Files]
Source: "Modelctl-Windows-x64\*"; DestDir: "{app}"; Flags: recursesubdirs createallsubdirs ignoreversion

[Icons]
Name: "{group}\Modelctl"; Filename: "{app}\Modelctl.exe"
Name: "{autodesktop}\Modelctl"; Filename: "{app}\Modelctl.exe"

[Run]
Filename: "{app}\Modelctl.exe"; Description: "Launch Modelctl"; Flags: nowait postinstall skipifsilent
