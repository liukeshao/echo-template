import "@typespec/http";
using TypeSpec.Http;

namespace GoRestTemplate;

model ListQuery {
  @doc("1-based page number (0 or omitted defaults to 1).")
  @query @minValue(0) page?: int32 = 1;
  @doc("Items per page (0 or omitted defaults to 50; values above 1000 are clamped to 1000).")
  @query @minValue(0) page_size?: int32 = 50;
  @doc("Comma-separated sort clauses, e.g. \"created_at desc, title asc\".")
  @query order_by?: string;
}

model Page<T> {
  items: T[];
  page: int32;
  page_size: int32;
  total_size: int32;
}
