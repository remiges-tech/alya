# wscutils additive API

This file documents the additive API introduced for Alya envelope style services.

## Existing API

These functions and behaviors are unchanged:

- `BindJSON`
- `WscValidate`
- `SetValidationTagToMsgIDMap`
- `SetValidationTagToErrCodeMap`
- package-level validation defaults and maps
- `NewSuccessResponse`
- `NewErrorResponse`
- `SendSuccessResponse`
- `SendErrorResponse`

Existing users can keep using the current API without changes.

## New additive API

These APIs are added for new envelope-style services that want:

- resource-oriented routes
- typed validator instances
- no package-level validation state for new code
- helper senders with explicit HTTP status handling

New APIs:

- `BindData`
- `ParseInt64PathParam`
- `SendOK`
- `SendCreated`
- `SendAccepted`
- `SendDeleted`
- `SendError`
- `Validator`
- `ValidationRule`

## When to use which API

Use the existing API when you need full backward compatibility with current Alya code and do not want to change validation setup.

Use the additive API for new Alya envelope services where you want:

- `{"data": ...}` request binding via `BindData`
- Alya envelope responses via `SendOK`, `SendCreated`, `SendDeleted`, and `SendError`
- instance-based validation via `Validator`
- Alya `msgid`, `errcode`, `field`, and `vals` generation without global validator maps

## Compatibility

- `BindJSON` is unchanged.
- `WscValidate` is unchanged.
- package-level validation maps are unchanged.
- existing callers keep current behavior.
