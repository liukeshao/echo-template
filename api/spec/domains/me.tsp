import "@typespec/http";
import "../common/scalars.tsp";
import "../common/problem.tsp";
import "../common/patch.tsp";

using TypeSpec.Http;
using GoRest;

namespace EchoTemplate;

model User {
  id: ULID;
  username: string;
  email: string;
  status: string;
  last_login_at?: utcDateTime;
  created_at: utcDateTime;
  updated_at: utcDateTime;
}

model ReplaceMeRequest {
  username: Username;
  email: Email;
}

model ChangePasswordRequest {
  @minLength(1) old_password: string;
  new_password: Password;
}

@route("/api/v1/me")
@tag("me")
@useAuth(BearerAuth)
interface Me {
  @doc("获取当前用户信息")
  @get
  get():
    | User
    | UnauthorizedProblem;

  @doc("替换当前用户资料（幂等全量更新）")
  @put
  replace(@body body: ReplaceMeRequest):
    | User
    | BadRequestProblem
    | UnauthorizedProblem
    | ConflictProblem;

  @doc("局部更新当前用户资料（支持 RFC 7386 Merge Patch / RFC 6902 JSON Patch / Shorthand）")
  @autoPatch(Me.get, Me.replace)
  @patch
  patch(
    @header contentType:
      | "application/merge-patch+json"
      | "application/json-patch+json"
      | "application/merge-patch+shorthand"
      | "application/json",
    @body body: Record<unknown> | JsonPatchOperation[] | string,
  ):
    | User
    | BadRequestProblem
    | UnauthorizedProblem
    | ConflictProblem
    | UnsupportedMediaTypeProblem;

  @doc("修改当前用户密码")
  @post
  @route("/change-password")
  changePassword(@body body: ChangePasswordRequest):
    | { @statusCode statusCode: 204 }
    | BadRequestProblem
    | UnauthorizedProblem;
}
