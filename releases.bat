@echo off
set PATH=%PATH%;C:\Program Files\7-Zip

echo Generating directory structure
mkdir tmp
copy /y settings_example.json tmp\
copy /y README.md tmp\

mkdir tmp\templates
copy /y templates tmp\templates

mkdir tmp\banners
copy /y banners\unknown.jpg tmp\banners

mkdir builds
del /f /q builds\*

echo Building Windows 386
set GOOS=windows
set GOARCH=386
go build -o tmp/steam-screenshots.exe

7z a builds\windows_386.zip .\tmp\*

echo Building Linux 386
del tmp\steam-screenshots.exe
set GOOS=linux
set GOARCH=386
go build -o tmp/steam-screenshots

7z a builds\linux_386.tar .\tmp\*
7z a builds\linux_386.tar.gz builds\linux_386.tar

echo Building Linux amd64
del tmp\steam-screenshots
set GOOS=linux
set GOARCH=amd64
go build -o tmp/steam-screenshots

7z a builds\linux_amd64.tar .\tmp\*
7z a builds\linux_amd64.tar.gz builds\linux_amd64.tar

echo Building Linux arm
del tmp\steam-screenshots
set GOOS=linux
set GOARCH=arm
go build -o tmp/steam-screenshots

7z a builds\linux_arm.tar .\tmp\*
7z a builds\linux_arm.tar.gz builds\linux_arm.tar

echo Packaging source
del tmp\steam-screenshots
copy *.go tmp
7z a builds\source.tar tmp\*
7z a builds\source.tar.gz builds\source.tar

@pause
echo Cleaning up
del builds\*.tar
rmdir /s /q tmp
rem del /s /f /q tmp

echo Done
