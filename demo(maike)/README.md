## A simple argo workflow demo

the workflow would create a pod in default namespace and use python image to run some code, the workflow template is defined as a json file and parsed in main.go

```
# to give permission to user to create job in pod

kubectl apply -f rolebinding.yaml
```

```
go run main.go
```