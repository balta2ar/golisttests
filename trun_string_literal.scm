(function_declaration
  name: (identifier) @func.name
  (#match? @func.name "^Test.*$")
  (parameter_list)
  (block
    (call_expression
      (selector_expression) @call.name
      (#match? @func.name "^Test.*$")
      (#match? @call.name "^t.Run$")
      (argument_list
        (interpreted_string_literal) @test.name
        (func_literal))))
  )
