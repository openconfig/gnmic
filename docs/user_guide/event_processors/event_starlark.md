### Intro

The `event-starlark` processor applies a [`Starlark`](https://github.com/google/starlark-go/blob/master/doc/spec.md) function on a list of `event` messages before returning them to the processors pipeline and then to the output.

`starlark` is a dialect of Python, developed initially for the [Bazel build tool](https://bazel.build/) but found multiple uses as a configuration language embedded in a larger application.

There are a few differences between Python and Starlark, programs written in Starlark are supposed to be short-lived and have no external side effects, their main result is structured data or side effects on the host application. As a result, Starlark has no need for classes, exceptions, reflection, concurrency, and other such features of Python.

`gNMIc` uses the [Go implementation](https://github.com/google/starlark-go/blob/master/doc/spec.md) of Starlark.

A Starlark program running as a `gNMIc` processor should define an `apply` function that takes an arbitrary number of arguments of type `Event` and returns zero or more `Event`s.

An [`Event`](intro.md#the-event-format) is the transformed gNMI update message as `gNMIc` processes it.

```python
def apply(*events)
  # events transformed/augmented/filtered here
  return events
```

### Configuration

```yaml
processors:
  # processor name
  sample-processor:
    # processor type
    event-starlark:
      # the source of the starlark program.
      source: |
        def apply(*events):
          # processor logic here
          return events
      # path to a file containing the starlark program to run.
      # Mutually exclusive with `source` parameter. 
      script:
      # boolean enabling extra logging
      debug: false
```

### Writing a Starlark processor

To write a starlark processor all that is needed is writing a function called `apply` that will read/modify/delete a list of `Event` messages.

Starlark defines multiple builtin types and functions, see spec [here](https://github.com/google/starlark-go/blob/d1966c6b9fcd/doc/spec.md)

There are some additional builtin functions like `Event(name)` which creates a new `Event` message and `copy_event(Event)` which duplicates a given `Event` message.

The `Event` message comprises a few fields:

- `name`: string

- `timestamp`: int64

- `tags`: dictionary of string to string

- `values`: dictionary of string to any

- `deletes`: list of strings

Two libraries are available for loading:

- **time**: `load("time.star", "time")` loads the time library which provides the following functions to work with the `Event` message timestamp field:
    - `time.from_timestamp(sec, nsec)`:

          Converts the given Unix time corresponding to the number of seconds
          and (optionally) nanoseconds since January 1, 1970 UTC into an object
          of type Time. 
          
          For more details, refer to https://pkg.go.dev/time#Unix.

    - `time.is_valid_timezone(loc)`:

          Reports whether loc is a valid time zone name.

    -  `time.now()`:

          Returns the current local time.

    -  `time.parse_duration(d)`:

          Parses the given duration string.
          
          For more details, refer to https://pkg.go.dev/time#ParseDuration.

    - `time.parseTime(x, format, location)`:

          Parses the given time string using a specific time format and location.
          The expected arguments are a time string (mandatory), a time format
          (optional, set to RFC3339 by default, e.g. "2021-03-22T23:20:50.52Z")
          and a name of location (optional, set to UTC by default). 
          
          For more details, refer to https://pkg.go.dev/time#Parse and https://pkg.go.dev/time#ParseInLocation.

    -  `time.time(year, month, day, hour, minute, second, nanosecond, location)`:

          Returns the Time corresponding to `yyyy-mm-dd hh:mm:ss + nsec nanoseconds` in the appropriate zone for that time
          in the given location. All the parameters are optional.

- **math**: `load("math.star", "math")` loads the math library which provides a set of constants and math-related functions:
    - `ceil(x)`:

          Returns the ceiling of x, the smallest integer greater than or equal to x.

    - `copysign(x, y)`:

         Returns a value with the magnitude of x and the sign of y.

    - `fabs(x)`:

         Returns the absolute value of x as float.

    - `floor(x)`:

         Returns the floor of x, the largest integer less than or equal to x.

    - `mod(x, y)`:

         Returns the floating-point remainder of x/y. The magnitude of the result is less than y and its sign agrees with that of x.

    - `pow(x, y)`:

         Returns x**y, the base-x exponential of y.

    - `remainder(x, y)`:

         Returns the IEEE 754 floating-point remainder of x/y.

    - `round(x)`:

         Returns the nearest integer, rounding half away from zero.

    - `exp(x)`:

         Returns e raised to the power x, where e = 2.718281… is the base of natural logarithms.

    - `sqrt(x)`:

         Returns the square root of x.

    - `acos(x)`:

         Returns the arc cosine of x, in radians.

    - `asin(x)`:

         Returns the arc sine of x, in radians.

    - `atan(x)`:
         
         Returns the arc tangent of x, in radians.

    - `atan2(y, x)`:

        Returns atan(y / x), in radians.
        The result is between -pi and pi.
        The vector in the plane from the origin to point (x, y) makes this angle with the positive X axis.
        The point of atan2() is that the signs of both inputs are known to it, so it can compute the correct
        quadrant for the angle.
        For example, atan(1) and atan2(1, 1) are both pi/4, but atan2(-1, -1) is -3*pi/4.

    - `cos(x)`:

        Returns the cosine of x, in radians.

    - `hypot(x, y)`:

        Returns the Euclidean norm, sqrt(x*x + y*y). This is the length of the vector from the origin to point (x, y).

    - `sin(x)`:

        Returns the sine of x, in radians.

    - `tan(x)`:

        Returns the tangent of x, in radians.

    - `degrees(x)`:

        Converts angle x from radians to degrees.

    - `radians(x)`:

        Converts angle x from degrees to radians.

    - `acosh(x)`:

        Returns the inverse hyperbolic cosine of x.

    - `asinh(x)`:
    
        Returns the inverse hyperbolic sine of x.

    - `atanh(x)`:
    
        Returns the inverse hyperbolic tangent of x.

    - `cosh(x)`:

        Returns the hyperbolic cosine of x.

    - `sinh(x)`:

        Returns the hyperbolic sine of x.

    - `tanh(x)`:

        Returns the hyperbolic tangent of x.

    - `log(x, base)`:

        Returns the logarithm of x in the given base, or natural logarithm by default.

    - `gamma(x)`:
    
        Returns the Gamma function of x.

### Examples

#### Move a value to a tag

```python
def apply(*events):
  dels = []
  for e in events:
    for k, v in e.values.items():
      if k == "val1":
        e.tags[k] = str(v)
        dels.append(k)
    for d in dels:
      e.values.pop(d)
  return events
```

#### Rename values

```python
val_map = {
  "val1": "new_val",
}

def apply(*events):
  for e in events:
    for k, v in e.values.items():
      if k in val_map:
        e.values[val_map[k]] = v
        e.values.pop(k)
  return events
```

#### Convert strings to integers

```python
def apply(*events):
  for e in events:
    for k, v in e.values.items():
      if v.isdigit():
        e.values[k] = int(v)
  return events
```

#### Set an interface description as a tag

This script stores each interface description per target/interface in a cache and
adds it to other values as a tag.

```python
cache = {}

def apply(*events):
  evs = []
  # check if on the event messages contains an interface description
  # and store in th cache dict
  for e in events:
    if e.values.get("/interface/description"):
      target_if = e.tags["source"] + "_" + e.tags["interface_name"]
      cache[target_if] = e.values["/interface/description"]
  # for each event get the 'source' and 'interface_name', check
  # if a corresponding cache entry exists and set it as a 
  # 'description' tag
  for e in events:
    if e.tags.get("source") and e.tags.get("interface_name"):
      target_if = e.tags["source"] + "_" + e.tags["interface_name"]
      if cache.get(target_if):
        e.tags["description"] = cache[target_if]
    evs.append(e)
  return evs
```

#### Calculate new values based on the received ones

The below script calculates the avg, min, max of a list of values over their last N=10 values

```python
cache = {}

values_names = [
  '/interface/statistics/out-octets',
  '/interface/statistics/in-octets'
]

N=10

def apply(*events):
  for e in events:
    for value_name in values_names:
      v = e.values.get(value_name)
      # check if v is not None and is a digit to proceed
      if not v.isdigit():
        continue
      # update cache with the latest value
      val_key = "_".join([e.tags["source"], e.tags["interface_name"], value_name])
      if not cache.get(val_key):
        # initialize the cache entry if empty
        cache.update({val_key: []})
      if len(cache[val_key]) >= N:
        # remove the oldest entry if the number of entries reached N
        cache[val_key] = cache[val_key][1:]
      # update cache entry
      cache[val_key].append(int(v))
      # get the list of values
      val_list = cache[val_key]
      # calculate min, max and avg
      e.values[value_name+"_min"] = min(val_list)
      e.values[value_name+"_max"] = max(val_list)
      e.values[value_name+"_avg"] = avg(val_list)
  return events

def avg(vals):
  sum = 0
  for v in vals:
    sum = sum + v
  return sum/len(vals)
```

The below script builds on top of the previous one by adding the rate calculation to the added values.
Now the cache contains a timestamp as well as the value.

```python
cache = {}

values_names=[
  '/interface/statistics/out-octets',
  '/interface/statistics/in-octets'
]

N=10

def apply(*events):
  for e in events:
    for value_name in values_names:
      v = e.values.get(value_name)
      # check if v is not None and is a digit to proceed
      if not v.isdigit():
        continue
      # update cache with the latest value
      val_key = "_".join([e.tags["source"], e.tags["interface_name"], value_name])
      if not cache.get(val_key):
        # initialize the cache entry if empty
        cache.update({val_key: []})
      if len(cache[val_key]) >= N:
        # remove the oldest entry if the number of entries reached N
        cache[val_key] = cache[val_key][1:]
      # update cache entry
      cache[val_key].append((e.timestamp, int(v)))
      # get the list of values
      val_list = cache[val_key]
      # calculate min, max and avg
      vals = [x[1] for x in val_list]
      e.values[value_name+"_min"] = min(vals)
      e.values[value_name+"_max"] = max(vals)
      e.values[value_name+"_avg"] = avg(vals)
      if len(val_list) > 1:
        e.values[value_name+"_rate"] = rate(val_list[-2:])
  return events

def avg(vals):
  sum = 0
  for v in vals:
    sum = sum + v
  return sum/len(vals)

def rate(vals):
  period = (vals[1][0] - vals[0][0]) / 1000000000
  change = vals[1][1] - vals[0][1]
  return change / period
```
