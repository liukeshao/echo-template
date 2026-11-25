import "@typespec/http";
import "@typespec/openapi";
import "tsp-emitter-go-rest";
import "../common/scalars.tsp";
import "../common/pagination.tsp";
import "../common/problem.tsp";

using TypeSpec.Http;
using TypeSpec.OpenAPI;
using GoRest;

namespace EchoTemplate;

model Book {
  id: UUID;
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
  @doc("Optional custom UUID. Auto-generated when omitted.")
  book_id?: UUID;
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

@doc("RFC 7386 JSON Merge Patch for Book (omit field to keep, set null to clear optional fields).")
model PatchBookRequest {
  @minLength(1) @maxLength(200) title?: string;
  @minLength(1) @maxLength(100) author?: string;
  @maxLength(1000) description?: string | null;
  @minValue(0) price?: float64 | null;
  published_at?: utcDateTime | null;
}

@route("/api/v1/books")
@tag("books")
@useAuth(BearerAuth)
interface Books {
  @doc("创建书籍")
  @operationId("createBook")
  @post
  createBook(@body body: CreateBookRequest):
    | { @statusCode statusCode: 201; @body body: Book }
    | BadRequestProblem
    | UnauthorizedProblem
    | ConflictProblem;

  @doc("获取书籍列表")
  @operationId("listBooks")
  @get
  listBooks(
    ...ListQuery,
    @doc("按书名模糊匹配") @query title?: string,
    @doc("按作者模糊匹配") @query author?: string,
    @doc("最低价格（含）") @query @minValue(0) min_price?: float64,
    @doc("最高价格（含）") @query @minValue(0) max_price?: float64,
  ):
    | BookList
    | BadRequestProblem
    | UnauthorizedProblem;

  @doc("获取书籍详情")
  @operationId("getBook")
  @get
  @route("/{book}")
  getBook(@path book: UUID):
    | Book
    | UnauthorizedProblem
    | NotFoundProblem;

  @doc("全量替换书籍（幂等 PUT）")
  @operationId("replaceBook")
  @put
  @route("/{book}")
  replaceBook(@path book: UUID, @body body: ReplaceBookRequest):
    | Book
    | BadRequestProblem
    | UnauthorizedProblem
    | NotFoundProblem;

  @doc("局部更新书籍（RFC 7386 JSON Merge Patch）")
  @operationId("patchBook")
  @autoPatch(Books.getBook, Books.replaceBook)
  @patch
  @route("/{book}")
  patchBook(
    @path book: UUID,
    @header contentType: "application/merge-patch+json" | "application/json",
    @body body: PatchBookRequest,
  ):
    | Book
    | BadRequestProblem
    | UnauthorizedProblem
    | NotFoundProblem
    | UnsupportedMediaTypeProblem;

  @doc("删除书籍")
  @operationId("deleteBook")
  @delete
  @route("/{book}")
  deleteBook(@path book: UUID):
    | { @statusCode statusCode: 204 }
    | UnauthorizedProblem
    | NotFoundProblem;
}
