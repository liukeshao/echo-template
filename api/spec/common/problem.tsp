import "@typespec/http";
using TypeSpec.Http;

namespace EchoTemplate;

model FieldError {
  field: string;
  message: string;
}

@error
@doc("RFC 9457 Problem Details")
model Problem {
  @header contentType: "application/problem+json";
  type: string = "about:blank";
  title: string;
  status: int32;
  detail: string;
  instance?: string;
  code: string;
  request_id?: string;
  params?: Record<string>;
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
