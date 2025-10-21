# Download Tensorflow from: https://www.tensorflow.org/install/lang_c
# Move extracted contents of './include => /usr/local/include'
# Move extracted contents of './lib     => /usr/local/lib'
# Update Library cache with 'sudo ldconfig'

export TF_CPP_MIN_LOG_LEVEL=2
export CGO_ENABLED=1
export CGO_CFLAGS="-I/usr/local/include"
export CGO_LDFLAGS="-L/usr/local/lib -ltensorflow"

go run main.go
