@echo off
REM Copy native Discord voice libraries to output directory

echo Copying opus.dll...
copy "%USERPROFILE%\.nuget\packages\opusdotnet.opus.win-x64\1.3.1\runtimes\win-x64\native\opus.dll" "bin\Debug\net10.0\" >nul

echo Copying libsodium.dll...
copy "%USERPROFILE%\.nuget\packages\libsodium\1.0.20\runtimes\win-x64\native\libsodium.dll" "bin\Debug\net10.0\" >nul

echo Done! Native libraries copied successfully.
