namespace EchoTemplate;

@doc("26-character ULID identifier")
@minLength(26)
@maxLength(26)
@pattern("^[0-9A-HJKMNP-TV-Z]{26}$")
scalar ULID extends string;

@doc("RFC 5322 email address")
@format("email")
scalar Email extends string;

@doc("Username (3-50 chars, letters/digits/underscore/hyphen)")
@minLength(3)
@maxLength(50)
@pattern("^[a-zA-Z0-9_-]+$")
scalar Username extends string;

@doc("Password (8-128 chars, must include at least one letter and one digit)")
@minLength(8)
@maxLength(128)
scalar Password extends string;
