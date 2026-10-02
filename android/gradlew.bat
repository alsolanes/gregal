@echo off
setlocal
set "APP_HOME=%~dp0"
set "JAVA_EXE=java.exe"
if not defined JAVA_HOME goto run
set "JAVA_EXE=%JAVA_HOME%\bin\java.exe"
if exist "%JAVA_EXE%" goto run
echo JAVA_HOME does not point to a Java installation. 1>&2
exit /b 1

:run
"%JAVA_EXE%" %JAVA_OPTS% %GRADLE_OPTS% -classpath "%APP_HOME%gradle\wrapper\gradle-wrapper.jar" org.gradle.wrapper.GradleWrapperMain %*
exit /b %ERRORLEVEL%
