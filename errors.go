package otelsql

import "go.opentelemetry.io/otel"

func handleErr(err error) {
	if err != nil {
		otel.Handle(err)
	}
}

func mustNoError(err error) {
	if err != nil {
		panic(err)
	}
}
