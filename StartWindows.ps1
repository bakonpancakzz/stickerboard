# Download Tensorflow from: https://www.tensorflow.org/install/lang_c
# Extract ZIP contents to 'C:\lib\tensorflow'
# Add 'C:\lib\tensorflow\lib' to System PATH

$env:TF_CPP_MIN_LOG_LEVEL=2
$env:CGO_ENABLED = "1"
$env:CGO_CFLAGS = "-IC:\lib\tensorflow\include"
$env:CGO_LDFLAGS = "-LC:\lib\tensorflow\lib -ltensorflow"
go run main.go