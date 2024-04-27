## Web services: basic rules

* Web service calls will expect requests using the `POST` method, not `GET`, typically. We will usually not design URLs to communicate request parameters. The only exception will be in "deep URL" cases where we need to supply a link or button somewhere which will take the user directly to a specific object instance within a module, for instance, the edit screen of a specific voucher within the FA module. In such cases, we can append the object ID to the URL, and use `GET`.
* All JSON data structures will have member names in full lowercase. No mixed-case will be used.
* We will use only HTTPS for all web services traffic. Calls must be set up to fail if HTTP is attempted; HTTP must not be redirected to HTTPS.
* HTTP response codes should not be examined by the caller to decide success or failure of the call -- any HTTP code in the range `200` to `299` should be accepted as success. HTTP protocol response codes are supposed to indicate protocol level and basic transport level issues -- not application level success or failure.

## Web service request format

This is one example:

``` json
{
    "data": {
        "goal_id": 23
    }
}
```

* **`data`** is the request data object. Each web service will define its own data object, with some mandatory and some optional fields. The exact contents of the `data` block, the exact member names, *etc*, will change from call to call. Here the example request has just one member: `goal_id`.

In addition, there will be three other parameters which will be supplied with (almost) all requests, (almost) always, irrespective of whether we use GET or POST. These are:
* the JWT: this token will be passed in the HTTP header. The server will receive the request, pull out the token, and validate it. Only if it is valid will the web service call code begin its work
* `ver`: an integer version number. This will go either in the HTTP header or in the URL (with a GET request). When a new call is released, it will expect `ver` to carry the value `1` always. As new and incompatible versions are released, the web service call will expect to receive `1`, or `2`, or `3`, *etc*, and switch to that version of its code. This allows multiple parallel versions of the code to serve multiple generations of front-end clients. If the `ver` is to be passed in the URL, it will be embedded in the following way: `https://app.datakatana.com/mis/v1/gettrialbalance?branch=402`. In this example, `mis` is the name of the application, and the request is hitting the `v1` release of the application, and in that, the access is to the `gettrialbalance` web service call, and it is being told to process the trial balance for branch `402`. This sort of URL can be used with a GET method. The `ver` value will need to be extracted from the URL and passed to the code as a header parameter.
* trace ID: an opaque string, hopefully unique in time and space. This will be set in the HTTP header by the client code, and will be passed to the web service call code as a parameter. It will be logged by the code on the server, and will be passed along for all inter-service calls, messages, and inter-module interaction, to track all the activities of one processing thread from another. A typical name for this header will be `X-CBSSOportal-Trace-ID`, where `CBSSOportal` is the name of the application being developed. So the full header line may be something like 

```
X-CBSSOportal-Trace-ID: cfb8ed3e-619f-401c-af6e-0e0a8e9a066d

```
where `CBSSOportal` is the name of the application, and the trace ID has been generated using UUID tools. (It's okay to use any other format instead of a UUID, provided it's highly likely to be unique in space and time.)

These are not shown in the request example, because they're not part of the *request body* but are in the *request header*. The JWT may be omitted in cases where the call is unauthenticated.

## Web service response format

A consistent JSON data structure will be returned by the API for success or errors. For successes, the following JSON structure will be returned:

``` javascript
{
    "status": "success",
    "data": {
        /* Application-specific data payload goes here. */
    },
    "messages": [] /* Or optional success message */
}
```

The contents of `data` for each web service call is provided in its detailed documentation. Note how the `message` block is an empty array here. The `status` attribute will always be returned in every response.

For errors, the JSON structure to be returned must be language independent, so that the client code can make sense of the error in an language-independent way, and still display a meaningful error message in the language of the end-user.

``` json
{
    "status": "error",
    "data": {},
    "messages": [{
        "errcode": "toobig",
        "msgcode": 235,
        "field": "maxdelay",
        "vals": [ "7", "3" ]
    }, {
        "errcode": "missing",
        "msgcode": 45,
        "field": "fullname"
    }]
}
```
The `data` block is empty in case of errors, and the error messages will be listed in `messages`.

The `data` block is an object, not an array. And the `messages` array is always an array, never an object.

In the `messages` array, the two mandatory fields are:
* `errcode`: a one-word string, all lower-case, giving the error code for the error. The list of error codes the system can send back is listed in the error dictionary. This code is useful for the client code programmer or front-end programmer -- she can see from the value of `errcode` what was the broad nature of the error, and take action. For instance, some types of error codes indicate internal system errors or statuses, like `auth` or `trylater`, whereas a lot of other error codes indicate errors in user-submitted data. The front-end code can distinguish these errors into these categories and act accordingly.
* `msgcode`: an integer, giving the index for the error message in human readable form. This index, combined with the language code the client software will apply based on the language being used in the front-end, will be used to pull out a message template string from a message dictionary. If, for instance, six languages are being supported by the front-end, then there will be six versions of errid `235` in the six languages.

The other two fields in each `messages` element are optional.
* `field`: gives a field name, indicating a field in the request which triggered the error. If the error is not field-specific, then this attribute will be omitted, *e.g.* in the case of `trylater` or `auth` errors.
* `vals`: will be an array of strings, giving one or more values which will be plugged into the message to be displayed to the end-user, as explained below in examples. If `field` is omitted, then `vals` will certainly be omitted.

And example of an error message object:
``` json
    {
        "errcode": "toobig",
        "msgcode": 235,
        "field": "maxdelay",
        "vals": [ "7", "3" ]
    }
```
This may be an error to indicate that a value of `7` was supplied in the field `maxdelay` in the request, where the max value permitted for this attribute is `3`. The error code is `toobig`, which tells the programmer that the general nature of the error is that some attribute in the request had a value which was too big for the server to accept. If you look up `msgid` 235 for English, you may find that it will be the string
```
"@<field>@ has the value @<val_0>@, exceeds maximum value @<val_1>@"
```
This tells the client code that it can pull out message 235 from the text dictionary, patch the value of `field` in the first placeholder, `vals[0]` in the second, `vals[1]` in the third, and display it to the user, and be fairly certain that the resulting message will be meaningful to the human. If this was an English language user, she would see
```
maxdelay has the value 7, exceeds maximum value 3
```
If message strings are accessed for `msgid` 235 for Bengali or Japanese, you may find the strings
```
@<field>@এর মান @<val_0>@, সর্বোচ্চ মান @<val_1>@ ছাড়িয়ে গেছে
@<field>@ の値は @<val_0>@ ですが、最大値 @<val_1>@ を超えています
```

After patching the three placeholders, the client code will generate
```
maxdelay এর মান 7, সর্বোচ্চ মান 3 ছাড়িয়ে গেছে
```
or
```
maxdelay の値は 7 ですが、最大値 3 を超えています
```
which is meaningful for the corresponding user.

It is possible that the UI message designer may decide to omit some or all of the placeholders from a message template (in one language, some languages, or all languages), and the `message` object will still be usable. For instance, the text template may say
```
Maximum batch delay has the value @<val_0>@, cannot exceed maximum value @<val_1>@
```

If the message object is processed in the front-end code and patched into this template, it will result in
```
Maximum batch delay has the value 7, cannot exceed maximum value 3
```

In the example above, the `@<field>@` tag was omitted, but the message block still works well and the resulting message is still meaningful. It's up to the message designer for the front-end.

A second example:

``` json
    {
        "errcode": "missing",
        "msgcode": 45,
        "field": "fullname"
    }
```
This may be an error to indicate that a mandatory attribute was missing in the request. The error code says `missing`, which clearly tells the client code software that the nature of the error is a missing mandatory field in the request, or a request which was trying to perform an operation on an object which was missing in the system (*e.g.* trying to edit voucher number 24124, where there is no voucher number 24124 in the system). The `msgcode` is 45, which will point to a string in the text dictionary. And the `field` field gives the name of the attribute which should have been present. The message string 45 may be
```
Mandatory field @<field>@ missing
```

## Interpreting the errors returned by web services

Each error reported has two parts:

* the error code: `errcode`
* the human readable error message: `msgcode`, `field`, `vals`

The `errcode` is always going to be one word, always in lower case English, with letters, digits, and/or underscore characters.

The `errcode` is the real indicator of the type of error, and is supposed to be used by the client code. It may handle the error, it may retry the operation, it may branch into a different path in the code, it may select a message to show the end-user, _etc_, as needed. And the code may also look at the value of `field` to understand whether the error was specific to a field or an overall error. The front-end programmer may choose to design his UI such that field-specific errors are displayed on the screen just below the corresponding input fields, in small red text, and so on.

The human readable error message, constructed out of the 3-tuple `(msgcode,field,vals)`, is supplied to the client code to use at its discretion. It may choose to display this message as-is to the human user, or log it using Sentry, or just ignore it and show a totally different message instead, as business logic and error handling strategy dictates. Some errors like `authexp` (see below) do not require any action on the human user's part -- they will just trigger a transparent re-generation of a new access token.

In multi-lingual user interfaces, the error message must be indexed to the combination of `(language, msgcode)` and the appropriate language-specific error message must be selected from a table and displayed to end-users. The responsibility of designing and selecting meaningful error messages in the correct language lies with the front-end software. The front-end software dev team must build error message tables in each supported language. **Why?** Because the server-side code does not need or use the language-specific errors -- these language-specific things are only for the benefit of the humans who use the front-end.

Common error codes are:
* `auth`: for login calls, this means that credentials are invalid. For all authentication calls, this error indicates that the user does not have access rights to perform the action she is attempting
* `authexp`: an access is being attempted with an access token which has expired. Typically, this is not a fatal error, but requires the front-end to use its refresh token to make a call to the system to generate a fresh access token. It becomes a fatal error if the refresh token too is found to have expired.
* `trylater`: the server is receiving too many requests from the client's IP address and is rate throttling the call (typically for unauthenticated calls). Or the server is processing a long-running request (like the generation of a large report), and has queued the request, but the report is not yet ready, therefore is signalling to the client to reattempt the same access later.
* `missing`: a mandatory parameter is missing in the request, or a key or ID has been supplied to access an object but there is no object with that ID in the system
* `toobig`: a parameter contains a value too high for the server to accept
* `toosmall`: the opposite of `toobig`
* `toonew`: a date field or timestamp field contains a date or timestamp which is too recent -- the system expects a value more in the past
* `tooold`: the opposite of `toonew`
* `toomany`: a list or array supplied in the request is too large -- it has too many values
* `exists`: an object creation or insertion is being attempted, with a specific value for a unique attribute, and there is an existing object in the system with that value. Example: a user creation is  being attempted, but another user with the same email address already exists, and the system has a policy  of not allowing multiple users to share a common email address.
* `datafmt`: the value supplied with a parameter in the request does not seem to be of the right format, *e.g.* an integer was expected but the value does not appear to be an integer, or a phone number was expected and there seems to be spurious alphabetic characters in the value, *etc*

These are **not** the exhaustive list of error codes possible, but are generic enough to apply to perhaps 95% of the errors actually returned by any application. There will always be additional error codes needed for highly specific types of errors.

Note that a single web service call may return a `messages` array with a whole bunch of messages, and multiple messages in that array may have the same `errcode`, *e.g.* `datafmt`. This means that there are data format errors detected with multiple fields in the request.

## Authentication

We will use token based authentication and session management for all applications henceforth, and will try hard to use [Remiges IDshield](https://github.com/remiges-tech/idshield/wiki) or, failing that, [Keycloak](https://www.keycloak.org/) for these functions. These products implement the [OAuth2 standard](https://oauth.net/2/) for authentication and session management. We will completely stop sending session IDs in cookies from servers, and stop keeping session tables in databases, which were common practices in the 20th century.

With these frameworks, the user table will be in a separate database, separate from the main application, and will be managed by IDshield. The login screen will be served to the front-end by IDshield directly, even in the case of PWA or mobile apps. When the user logs in, a handle will be returned to the front-end, which can then be used to fetch the access and refresh tokens from IDshield, and web service calls can then be accessed using the access token.

The web service call must validate the JWT signature and check the expiry date to decide whether the session is valid. The front-end code must supply the JWT in the header of every request and must have the functionality to handle the token expiry error and make the call to regenerate the access token by using the refresh token.

User management is not directly related to authentication, nevertheless one aspect of using IDshield must be noted here. The user table is managed by IDshield. This means that the business application must make calls to IDshield to do CRUD operations on users.

## API versions

Any web service (we will refer to it as "service" here) which is exposed to multiple clients (software systems which send web services requests to our service) will, over time, have to serve old clients and new ones.

Our service will grow in features and functionality, and the semantics of some web services may change. For instance, a particular web service may have been released initially with just three parameters in its input, but a fourth parameter was felt necessary after two years of service. Some older clients will still access the web service with three parameters and newer clients will pass four parameters to it. _C'est la vie_. One of the most obvious examples of this is when mobile apps access a web service. Some phones will have older releases of the mobile app and others will have newer releases. This makes it necessary for the web service to serve all generations of clients without breaking any.

One way to handle this is by adding an "API version number" parameter to each web service. This will be a mandatory parameter from Day 1. Initial versions of clients will all be designed to use **`"ver": 1`** in their requests. When the software on the server-side application is upgraded and the semantics of a web service change, then it will support the old semantics under **`"ver" = 1`** and newer semantics with **`"ver" = 2`**. Newer clients which are aware of the updated semantics will use **`ver=2`** in their requests, with the new semantics. Older clients will not be aware of any changes.

The version number applies to each web service in isolation. The service as a whole may have versions 1 and 2 supported for some calls, 1, 2 and 3 for other calls, and just 1 for calls which have not changed at all.

This will go on, and new generations of web services will keep getting released. This will allow a single service to service multiple generations of clients at the same time.
