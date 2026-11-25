import "@typespec/http";
import "@typespec/openapi";
import "../common/scalars.tsp";
import "../common/problem.tsp";

using TypeSpec.Http;
using TypeSpec.OpenAPI;

namespace EchoTemplate;

model RegisterRequest {
  username: Username;
  email: Email;
  password: Password;
}

model LoginRequest {
  email: Email;
  @minLength(1) password: string;
}

model RefreshTokenRequest {
  @minLength(1) refresh_token: string;
}

model TokenPair {
  access_token: string;
  refresh_token: string;
  expires_at: int64;
}

@route("/api/v1/auth")
@tag("auth")
interface Auth {
  @doc("用户注册")
  @operationId("register")
  @post
  @route("/register")
  register(@body body: RegisterRequest):
    | { @statusCode statusCode: 201; @body body: TokenPair }
    | BadRequestProblem
    | ConflictProblem;

  @doc("用户登录")
  @operationId("login")
  @post
  @route("/login")
  login(@body body: LoginRequest):
    | TokenPair
    | BadRequestProblem
    | UnauthorizedProblem
    | ForbiddenProblem;

  @doc("刷新访问令牌")
  @operationId("refreshToken")
  @post
  @route("/refresh")
  refreshToken(@body body: RefreshTokenRequest):
    | TokenPair
    | BadRequestProblem
    | UnauthorizedProblem
    | ForbiddenProblem;

  @doc("用户登出（撤销该用户全部活跃令牌）")
  @operationId("logout")
  @useAuth(BearerAuth)
  @post
  @route("/logout")
  logout():
    | { @statusCode statusCode: 204 }
    | UnauthorizedProblem;
}
