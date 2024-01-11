## A simple argo workflow demo

the workflow would create 3 pods in default namespace and use python image to run some code, the workflow template is defined as a yaml file and parsed in main.go

```
# to give permission to user to create job in pod

kubectl apply -f rolebinding.yaml
```

```
go run main.go
```