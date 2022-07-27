# Packages Github repository re-structure

## Current Structure

```
├── api
│   ├── filereader.go
│   ├── filereader_test.go
...
│   └── v1alpha1
...
│       ├── package.go
│       ├── package_test.go
│       ├── package_types.go
│       ├── package_webhook.go
│       ├── packagebundle.go
│       ├── packagebundle_test.go
│       ├── packagebundle_types.go
│       ├── packagebundle_webhook.go
│       ├── packagebundlecontroller.go
│       ├── packagebundlecontroller_test.go
│       ├── packagebundlecontroller_types.go
│       ├── packagebundlecontroller_webhook.go
│       ├── packagebundlecontroller_webhook_test.go
│       ├── testutil.go
│       ├── webhook_suite_test.go
│       └── zz_generated.deepcopy.go
├── charts
...
├── config
│   ├── certmanager
...
├── controllers
...
│   ├── package_controller.go
...
├── generatebundlefile
...
├── pkg
│   ├── artifacts
│   ├── authenticator
│   ├── bundle
│   │   ├── client.go
│   ├── driver
│   ├── packages
│   │   ├── manager.go
│   │   ├── manager_test.go
│   │   └── mocks
│   ├── signature
│   └── testutil
...
└── scripts
└── common.sh
```

## Problem Statement

With the current repo structure, there is no clearly defined import structure. Currently `pkg` uses `api` for extracting types. Typically `pkg` would contain the business logic required to fulfill the requirements of the application hence this would dictate the `api` would potentially need to use `pkg` as a dependency. Therefore this would lead to an import cycle since `pkg -> api` and `api -> pkg` would potentially be needed.

## Potential Solution

```
├── api
│   ├── filereader.go
│   ├── filereader_test.go
...
│   └── v1alpha1
...
│       ├── package_webhook.go
│       ├── packagebundle_webhook.go
│       ├── packagebundlecontroller_webhook.go
│       ├── packagebundlecontroller_webhook_test.go
│       ├── testutil.go
│       ├── webhook_suite_test.go
│       └── zz_generated.deepcopy.go
├── charts
...
├── config
│   ├── certmanager
...
├── controllers
...
│   ├── package_controller.go
...
├── generatebundlefile
...
├── pkg
│   ├── artifacts
│   ├── authenticator
│   ├── bundle
│   │   ├── client.go
│   ├── driver
│   ├── packages
│   │   ├── manager.go
│   │   ├── manager_test.go
│   │   └── mocks
│   ├── signature
│   └── testutil
│   └── types
│       ├── package.go
│       ├── package_test.go
│       ├── package_types.go
│       ├── packagebundle.go
│       ├── packagebundle_test.go
│       ├── packagebundle_types.go
│       ├── packagebundlecontroller.go
│       ├── packagebundlecontroller_test.go
│       └── packagebundlecontroller_types.go
...
└── scripts
└── common.sh
```

### Analysis

This approach dictates the creation of a types directory under pkg and moving any types related items to this directory. This approach would allow the direction of imports to go one way instead of loosely defined structure. The direction of imports would be:
`api -> pkg`
`controllers -> pkg`
