@echo off
echo Packaging Figma Deliver Plugin...

REM Create a zip file containing all necessary files
powershell -Command "Compress-Archive -Path manifest.json, src, ui -DestinationPath figma-deliver-plugin.zip -Force"

echo Plugin packaged as figma-deliver-plugin.zip
echo.
echo Instructions for installation:
echo 1. Open Figma desktop app
echo 2. Go to Plugins > Development > Import plugin from manifest
echo 3. Select the manifest.json file from the extracted zip
echo.
echo Done!
