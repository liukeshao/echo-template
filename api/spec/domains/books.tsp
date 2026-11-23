import "@typespec/http";
import "../common/scalars.tsp";
import "../common/pagination.tsp";
import "../common/problem.tsp";
import "../common/patch.tsp";

using TypeSpec.Http;
using GoRest;

namespace EchoTemplate;

model Book {
  id: ULID;
  title: string;
  author: string;
  description?: string;
  price?: float64;
  published_at?: utcDateTime;
  created_at: utcDateTime;
  updated_at: utcDateTime;
}

model BookList is Page<Book>;

model CreateBookRequest {
  @doc("Optional custom ULID. Auto-generated when omitted.")
  book_id?: ULID;
  @minLength(1) @maxLength(200) title: string;
  @minLength(1) @maxLength(100) author: string;
  @maxLength(1000) description?: string;
  @minValue(0) price?: float64;
  published_at?: utcDateTime;
}

model ReplaceBookRequest {
  @minLength(1) @maxLength(200) title: string;
  @minLength(1) @maxLength(100) author: string;
  @maxLength(1000) description?: string;
  @minValue(0) price?: float64;
  published_at?: utcDateTime;
}

@route("/api/v1/books")
@tag("books")
@useAuth(BearerAuth)
interface Books {
  @doc("创建书籍")
  @post
  create(@body body: CreateBookRequest):
    | { @statusCode statusCode: 201; @body body: Book }
    | BadRequestProblem
    | UnauthorizedProblem
    | ConflictProblem;

  @doc("获取书籍列表")
  @get
  list(...ListQuery):
    | BookList
    | BadRequestProblem
    | UnauthorizedProblem;

  @doc("获取书籍详情")
  @get
  @route("/{book}")
  get(@path book: ULID):
    | Book
    | UnauthorizedProblem
    | NotFoundProblem;

  @doc("全量替换书籍（幂等 PUT）")
  @put
  @route("/{book}")
  replace(@path book: ULID, @body body: ReplaceBookRequest):
    | Book
    | BadRequestProblem
    | UnauthorizedProblem
    | NotFoundProblem;

  @doc("局部更新书籍（支持 RFC 7386 Merge Patch / RFC 6902 JSON Patch / Shorthand）")
  @autoPatch(Books.get, Books.replace)
  @patch
  @route("/{book}")
  patch(
    @path book: ULID,
    @header contentType:
      | "application/merge-patch+json"
      | "application/json-patch+json"
      | "application/merge-patch+shorthand"
      | "application/json",
    @body body: Record<unknown> | JsonPatchOperation[] | string,
  ):
    | Book
    | BadRequestProblem
    | UnauthorizedProblem
    | NotFoundProblem
    | UnsupportedMediaTypeProblem;

  @doc("删除书籍")
  @delete
  @route("/{book}")
  delete(@path book: ULID):
    | { @statusCode statusCode: 204 }
    | UnauthorizedProblem
    | NotFoundProblem;
}
