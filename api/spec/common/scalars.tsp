namespace GoRestTemplate;

@doc("RFC 9562 UUID identifier")
@format("uuid")
@minLength(36)
@maxLength(36)
scalar UUID extends string;

@doc("RFC 5322 email address")
@format("email")
@maxLength(255)
scalar Email extends string;

@doc("Username (3-50 chars, letters/digits/underscore/hyphen)")
@minLength(3)
@maxLength(50)
@pattern("^[a-zA-Z0-9_-]+$")
scalar Username extends string;

@doc("Password (8-72 bytes, must include at least one letter and one digit)")
@minLength(8)
@maxLength(72)
scalar Password extends string;
