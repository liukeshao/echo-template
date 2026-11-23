import "@typespec/http";
using TypeSpec.Http;

namespace EchoTemplate;

model ListQuery {
  @query @minValue(0) page?: int32 = 1;
  @query @minValue(0) @maxValue(1000) page_size?: int32 = 50;
  @query order_by?: string;
  @query filter?: string;
}

model Page<T> {
  items: T[];
  page: int32;
  page_size: int32;
  total_size: int32;
}
