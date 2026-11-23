import "@typespec/http";
import "../common/scalars.tsp";
import "../common/problem.tsp";

using TypeSpec.Http;

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
  @post
  @route("/register")
  register(@body body: RegisterRequest):
    | TokenPair
    | BadRequestProblem
    | ConflictProblem;

  @doc("用户登录")
  @post
  @route("/login")
  login(@body body: LoginRequest):
    | TokenPair
    | BadRequestProblem
    | UnauthorizedProblem
    | ForbiddenProblem;

  @doc("刷新访问令牌")
  @post
  @route("/refresh")
  refreshToken(@body body: RefreshTokenRequest):
    | TokenPair
    | BadRequestProblem
    | UnauthorizedProblem
    | ForbiddenProblem;

  @doc("用户登出（撤销该用户全部活跃令牌）")
  @useAuth(BearerAuth)
  @post
  @route("/logout")
  logout():
    | { @statusCode statusCode: 204 }
    | UnauthorizedProblem;
}
