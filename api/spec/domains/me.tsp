import "@typespec/http";
import "@typespec/openapi";
import "tsp-emitter-go-rest";
import "../common/scalars.tsp";
import "../common/problem.tsp";

using TypeSpec.Http;
using TypeSpec.OpenAPI;
using GoRest;

namespace GoRestTemplate;

enum UserStatus {
  active: "active",
  inactive: "inactive",
  suspended: "suspended",
}

model User {
  id: UUID;
  username: string;
  email: string;
  status: UserStatus;
  last_login_at?: utcDateTime;
  created_at: utcDateTime;
  updated_at: utcDateTime;
}

model ReplaceMeRequest {
  username: Username;
  email: Email;
}

@doc("RFC 7386 JSON Merge Patch for current user profile (omit field to keep).")
model PatchMeRequest {
  username?: Username;
  email?: Email;
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
  @operationId("getMe")
  @get
  getMe():
    | User
    | UnauthorizedProblem;

  @doc("替换当前用户资料（幂等全量更新）")
  @operationId("replaceMe")
  @put
  replaceMe(@body body: ReplaceMeRequest):
    | User
    | BadRequestProblem
    | UnauthorizedProblem
    | ConflictProblem;

  @doc("局部更新当前用户资料（RFC 7386 JSON Merge Patch）")
  @operationId("patchMe")
  @autoPatch(Me.getMe, Me.replaceMe)
  @patch
  patchMe(
    @header contentType: "application/merge-patch+json" | "application/json",
    @body body: PatchMeRequest,
  ):
    | User
    | BadRequestProblem
    | UnauthorizedProblem
    | ConflictProblem
    | UnsupportedMediaTypeProblem;

  @doc("修改当前用户密码")
  @operationId("changePassword")
  @post
  @route("/change-password")
  changePassword(@body body: ChangePasswordRequest):
    | { @statusCode statusCode: 204 }
    | BadRequestProblem
    | UnauthorizedProblem;
}
