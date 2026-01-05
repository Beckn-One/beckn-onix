gen schema:
npx ondc-code-generator schema -c ./config/ondc-workbench-onix/ondc.yaml -o ./schemas/ondc_ret10/v1.2.0 -f json
go build -buildmode=plugin -o ondcvalidator.so ./cmd/plugin.go

gen ondcValidations:
npx ondc-code-generator xval -c ./config/ondc-workbench-onix/ondc.yaml -o ./pkg/plugin/implementation/ondcValidator/ -l golang

TODOS:

1. create unit tests
2. update documentation
3. api service
4. logger to loki
