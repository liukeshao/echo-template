import "@typespec/http";
using TypeSpec.Http;

namespace GoRestTemplate;

model FieldError {
  field: string;
  code: string;
  message: string;
  metadata?: Record<string>;
}

@doc("AIP-193 ResourceInfo: describes a missing or conflicting resource.")
model ResourceInfo {
  resource_type: string;
  resource_name: string;
  field?: string;
}

@doc("AIP-193 PreconditionInfo: describes a failed precondition or credential state.")
model PreconditionInfo {
  type: string;
  subject?: string;
  detail?: string;
}

@doc("AIP-193 RetryInfo: describes rate limiting and suggested retry window.")
model RetryInfo {
  service: string;
  retry_after?: string;
}

@doc("Summary metadata for field validation failures.")
model ValidationInfo {
  error_count: int32;
}

@doc("Metadata for JSON request body decoding errors.")
model DecodeInfo {
  reason: string;
  offset?: int64;
}

@doc("Metadata for PATCH document or media type errors.")
model PatchInfo {
  content_type: string;
  supported?: string[];
  reason?: string;
  field?: string;
}

@doc("Metadata for HTTP 405 Method Not Allowed responses.")
model MethodInfo {
  method: string;
  path: string;
  allowed?: string[];
}

@doc("Metadata for HTTP 413 Content Too Large responses.")
model RequestLimitInfo {
  max_bytes: int64;
}

alias ErrorMetadata =
  | ResourceInfo
  | PreconditionInfo
  | RetryInfo
  | ValidationInfo
  | DecodeInfo
  | PatchInfo
  | MethodInfo
  | RequestLimitInfo
  | Record<unknown>;

@error
@doc("RFC 9457 Problem Details")
model Problem {
  @header contentType: "application/problem+json";
  type: string = "about:blank";
  title: string;
  status: int32;
  detail: string;
  instance?: string;
  request_id?: string;
  metadata?: ErrorMetadata;
  errors?: FieldError[];
}

alias BadRequestProblem = {
  @statusCode statusCode: 400;
  ...Problem;
};

alias UnauthorizedProblem = {
  @statusCode statusCode: 401;
  ...Problem;
};

alias ForbiddenProblem = {
  @statusCode statusCode: 403;
  ...Problem;
};

alias NotFoundProblem = {
  @statusCode statusCode: 404;
  ...Problem;
};

alias ConflictProblem = {
  @statusCode statusCode: 409;
  ...Problem;
};

alias UnsupportedMediaTypeProblem = {
  @statusCode statusCode: 415;
  ...Problem;
};
