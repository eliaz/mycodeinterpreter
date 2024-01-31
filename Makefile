build:
	docker build --build-arg NGROK_AUTHTOKEN=$(NGROK_AUTHTOKEN) -t mycodeinterpreter .
