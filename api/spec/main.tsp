import "@typespec/http";
import "./domains/auth.tsp";
import "./domains/me.tsp";
import "./domains/books.tsp";

using TypeSpec.Http;

@service(#{ title: "Go REST Template API" })
@server("http://localhost:8000", "Local development server")
namespace GoRestTemplate;
