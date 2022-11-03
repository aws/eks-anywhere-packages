# EKS-A Curated Packages End-to-End Test Framework Design

*The following is an excerpt from a larger design document that covers all aspects of curated packages end-to-end testing. These sections are provided for early comment and revision. — ewwolles@amazon.com*

- [Test Variables](#org8dd3115)
  - [Operations](#org802cd07)
  - [Value Types](#org58512af)
  - [Value Domains](#orgecd6ea6)
  - [Recognized Test Dimensions](#org666546a)
  - [Worst-case Cardinality](#orge08c449)
  - [Implementation Design Ideas](#orgc40bdd7)

<a id="#org8dd3115"></a>

# Test Variables
Many variables will define an end-to-end test run. These variables are called dimensions, and the goal of a testing system is to run tests that cover as much of the range of each dimension as possible. For some types of values, every value can be covered by tests, but for other types of values, one has to be content with some subset of values from an infinite range. The recognized dimensions for curated packages end-to-end tests are categorized and discussed below.


<a id="org802cd07"></a>

## Operations

Operations determine if a test should or shouldn't be run in a given environment<sup><a id="fnr.1" class="footref" href="#fn.1" role="doc-backlink">1</a></sup>. That is, for a given set of test dimension values, should test 'A' be run? Consequently, most supported operations will be predicates—operations that result in boolean values. Implemented operations may include the following broad categorizations:


<a id="org80db6dd"></a>

### Boolean comparators

<, ≤, ≥, >, =, ≠


<a id="orgded2dd9"></a>

### Set membership

⊂, ⊄, ∩, ∪


<a id="org58512af"></a>

## Value Types

A value's type categorizes its representation, and defines the behavior of operations between values of the same or different types. These types are very similar to those found in programming languages; they include number, string, boolean, etc.


<a id="org3d7fa3b"></a>

### String

String values are mainly used for abstract values that can be matched and compared. String values don't necessarily have an ordering. For example, what would it mean for a value to be less than "Red Hat"? Any ordering defined between the string values "Ubuntu" and "Red Hat" is external to the values themselves. There is little value in running tests that exclude providers > "J". Running a test where provider ∈ { "Red Hat", "Ubuntu" } could be useful.


<a id="org543a723"></a>

#### Possible Operations

-   Boolean comparison operators (lexicographically, though not likely useful)
-   Set membership operators


<a id="org2ed469a"></a>

### Semantic Version (semver)

Semantic versioning, as defined at [semver.org](https://semver.org).


<a id="org9d1dbe8"></a>

#### Possible Operations

-   Boolean comparison operators
-   Set membership operators


<a id="orgecd6ea6"></a>

## Value Domains

Two value domains are recognized at this time:

-   **Discreet:** A set of distinct and countable values. For example, years of schooling, or computer architectures.
-   **Continuous:** Any of an infinite number of values within a range. For example, a timed duration, or a file size.

At first glance, the complications involved in supporting continuous values can seem daunting. In practice howerver, the number of distinct values encountered are generally on the order of 2–100 and so for the purposes of an end-to-end test can usually be handled like discreet values. As an example, a duration has an infinite number of possible values, but for the purposes of an end-to-end test they can be rounded to a coarser-grained precision, effectively mapping them to discreet values. In other cases, like a semantic version, the number of possible values is infinite, but the number of supported values is likely much smaller, and discreet.

> It's not clear if value domains will have any impact on the test framework's implementation, but it could be useful in reasoning about or implementing comparison operations.


<a id="org666546a"></a>

## Recognized Test Dimensions

These dimensions have been identified as having an impact on end-to-end tests, and are therefore candidates for implementation in the system.


<a id="org65dd08b"></a>

### Provider

|                      |          |
|----------------------|----------|
| Type                 | string   |
| Domain               | discreet |
| Cardinality estimate | < 10     |


Values:

-   Bare Metal
-   CloudStack
-   Docker
-   Snowball
-   vSphere


<a id="org0df1bd1"></a>

### Provider Version

|                      |                                                        |
|----------------------|--------------------------------------------------------|
| Type                 | string                                                 |
| Domain               | continuous                                             |
| Cardinality estimate | < 5 per provider ||
| Note                 | The values are likely to be similar to semver.         |

> Are different provider versions supported? If so, which ones? How will we test on different versions of vSphere? What constitutes a different version of Bare Metal? Does a version even mean anything when running EKS-A on CloudStack?

| Provider   | Versions Supported         |
|------------|----------------------------|
| Bare Metal | N/A                        |
| CloudStack | 4.14+                      |
| Docker     | ? (TBD)                    |
| Snowball   | all (which is 1 right now) |
| vSphere    | v7+ (major releases only?) |


<a id="org1f576cb"></a>

### CLI Version

|                      |                                                                     |
|-------------------- |------------------------------------------------------------------- |
| Type                 | string                                                              |
| Domain               | continuous                                                          |
| Cardinality estimate | < 5 (It's unlikely that more than five versions would be supported) |

> Should a git tag be an option? Can a git tag be meaningfully compared to a string version? Should this include some kind of "run from macOS" option?


<a id="orgeae63fb"></a>

### Kubernetes Version

|                      |                                                                                                                                                         |
|-------------------- |------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Type                 | semver                                                                                                                                                  |
| Domain               | continuous                                                                                                                                              |
| Cardinality estimate | 5 (It's unlikely we'd support more than five versions at a time)                                                                                        |
| Note                 | As defined at [semver.org](https://semver.org). However, this could be implemented as a discreet value and limited to the versions that EKS-A supports. |

Values:

-   1.20
-   1.21
-   1.22
-   1.23
-   1.24


<a id="org1d8d676"></a>

### Node OS

|                      |                                     |
|-------------------- |----------------------------------- |
| Type                 | string                              |
| Domain               | discreet                            |
| Cardinality estimate | < 10                                |

Values:

-   Bottlerocket
-   Red Hat
-   Ubuntu


<a id="org686cd6e"></a>

### Node OS Version

|                      |                                             |
|----------------------|---------------------------------------------|
| Type                 | string                                      |
| Domain               | continuous                                  |
| Cardinality estimate | < 5 (per Node OS)                           |

Values:

-   Bottlerocket Versions
    -   TBD
-   Red Hat Versions
    -   TBD
-   Ubuntu Versions
    -   18.04 LTS (is supported by Canonical [until Apr-2023], does EKS-A support it?)
    -   20.04 LTS
    -   22.04 LTS


<a id="org17e178e"></a>

### Packages Controller

|                      |                                                                                                              |
|-------------------- |------------------------------------------------------------------------------------------------------------ |
| Type                 | semver                                                                                                       |
| Domain               | continuous                                                                                                   |
| Cardinality estimate | < 5 (It's unlikely we'd support more than 5 versions at a time,         but custom runs could add a one-off) |
| Note                 | This is semver, since helm charts require semver, and the controller is distributed as a helm chart.         |

> Should this also include a git tag? Can a semver be compared meaningfully to a semver?


<a id="org285d366"></a>

### Package

|                      |                                                                                                  |
|-------------------- |------------------------------------------------------------------------------------------------ |
| Type                 | semver                                                                                           |
| Domain               | continuous                                                                                       |
| Cardinality estimate | < 5 (It's unlikely we'd support more than 5 versions at a time,                                  |
| Note                 | This is semver, since helm charts require semver, and packages are distributed  as a helm chart. |

> Should this also include a git tag? Can a semver be compared meaningfully to a semver?


<a id="org297f82f"></a>

### Architecture

|                      |                                                                |
|-------------------- |-------------------------------------------------------------- |
| Type                 | string                                                         |
| Domain               | discreet                                                       |
| Cardinality estimate | 2 (I've no reason to think that RISC V is even on the horizon) |
| Note                 | Only amd64 and arm64 are supported at this time.               |

Test machine architecture is a good candidate for later implementation. It is unclear even if any non-amd64 machines are available for automated testing at this time anyway.


<a id="orge9549bd"></a>

### Self- Versus Remotely-managed

|                      |          |
|-------------------- |-------- |
| Type                 | string   |
| Domain               | discreet |
| Cardinality estimate | 2        |

Self-managed clusters are a good candidate for later implementation.


<a id="org1e1448b"></a>

### Bundle Version

|                      |                                                                                         |
|-------------------- |--------------------------------------------------------------------------------------- |
| Type                 | string                                                                                  |
| Domain               | discreet<sup><a id="fnr.2" class="footref" href="#fn.2" role="doc-backlink">2</a></sup> |
| Cardinality estimate | 2<sup><a id="fnr.3" class="footref" href="#fn.3" role="doc-backlink">3</a></sup>        |

Testing packages against an older bundle doesn't make sense. But it could be useful for testing that the controller can deal with changes in the format of the bundle itself over time.


<a id="orge08c449"></a>

## Worst-case Cardinality

The maximum number of test dimension permutations is likely to be greater than that which can be run in a reasonable amount of time. Below is a worst-case scenario:

|           |                              |
|--------- |---------------------------- |
| 5         | Providers                    |
| 5         | Provider versions            |
| 5         | CLI versions                 |
| 5         | Kubernetes versions          |
| 10        | Node OSes                    |
| 5         | Node OS versions             |
| 5         | Packages controller versions |
| 5         | Package versions             |
| 2         | Architectures                |
| 2         | Self vs Remotely managed     |
| 2         | Bundle version               |
| 6,250,000 | Total permutations           |

With 6,250,000 permutations, running on 32 CPUs at a rate of one test per CPU per minute, a complete test run would take ≈ 4½ months.

A more middle-of-the-road estimate might be:

|       |                              |
|-------|------------------------------|
| 5     | Providers                    |
| 1.6   | Provider versions            |
| 2     | CLI versions                 |
| 4     | Kubernetes versions          |
| 3     | Node OSes                    |
| 2     | Node OS versions             |
| 2     | Packages controller versions |
| 2     | Package versions             |
| 1     | Architectures                |
| 1     | Self vs Remotely managed     |
| 1     | Bundle version               |
| 1,536 | Total permutations           |

With this far more tractable scenario, a complete run would take forty-eight minutes. Of course this still assumes 32 CPUs available and able to run any test (none of which take more than a minute from beginning to end).

<style>
.markdown-body #worst-case-cardinality ~ table tbody tr,
.markdown-body #worst-case-cardinality ~ table tbody tr.even td,
.markdown-body #worst-case-cardinality ~ table tbody tr.even th,
.markdown-body #worst-case-cardinality ~ table tbody tr.odd td,
.markdown-body #worst-case-cardinality ~ table tbody tr.odd th {
    background-color: inherit;
    border: none;
}

.markdown-body #worst-case-cardinality ~ table tbody tr td {
    padding: 0 0.5em;
}

.markdown-body #worst-case-cardinality ~ table td:first-child {
    text-align: right;
}

.markdown-body #worst-case-cardinality ~ table tbody tr:last-child {
    border-top: 1px solid #c9d1d9;

}

.markdown-body #worst-case-cardinality ~ table tbody tr:last-child td {
    padding-top: 5px;
}
</style>


<a id="orgc40bdd7"></a>

## Implementation Design Ideas


<a id="orgf548cac"></a>

### Go tags via comments

Tests could be marked with go tags, and included or excluded based on their matching a run specification:

```go
// e2e:providers=["vSphere", "Docker"] 
// e2e:nodeOS!=["Red Hat"] 
func TestSomePackage(t *testing.T) { 
	assert.True(t, true) 
} 
```

This approach makes tests easy to identify, and is simple for test writers to implement. However there's added complexity in writing a tool to read, parse, and react to the go tags. The tags themselves are a bit magical, and will depend on documentation for discovery.


<a id="orgdbecf99"></a>

### Test functions to build state

```go 
func TestSomePackage(t \*testing.T) { 
    e2eTest := e2e.New(t) 
	e2e.IncludeProviders("vSphere", "Docker") 
	e2e.ExcludeNodeOS("Red Hat")
	assert.True(t, true)
}
```

This would be easier to implement, but is less elegant, and harder to automate test runs. For example, it would be tricky to write a tool that lists all test methods that can be run for any given provider.

While the methods to call for a given requirement are easier to discover (simply read the code for the e2e package), They clutter up the tests themselves.

## Footnotes

<sup><a id="fn.1" class="footnum" href="#fnr.1">1</a></sup> This term "environment" might be good to use when defining how a run filters dimensions.

<sup><a id="fn.2" class="footnum" href="#fnr.2">2</a></sup> While technically continuous, only a limited number of bundles would be supported at any time, making this behave like a discreet variable.

<sup><a id="fn.3" class="footnum" href="#fnr.3">3</a></sup> Current and previous, in keeping with a rolling release strategy.
