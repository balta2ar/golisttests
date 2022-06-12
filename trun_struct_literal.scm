(function_declaration
  name: (identifier) @func.name
  (#match? @func.name "^Test.*$")
  (parameter_list)
  (block
    (short_var_declaration
      left: (expression_list
              (identifier) @var.name)
      (#match? @var.name "^tests$")
      right: (expression_list
               (composite_literal
                 type: (slice_type
                         element: (struct_type
                                    (field_declaration_list
                                      (field_declaration
                                        name: (field_identifier) @test.field.type.name
                                        (#match? @test.field.type.name "^name$")
                                        type: (type_identifier) @test.field.type
                                        (#match? @test.field.type "^string$")
                                        ))))
                 body: (literal_value
                         (element
                           (literal_value
                             (keyed_element
                               (field_identifier) @test.field.literal.name
                               (#eq? @test.field.literal.name @test.field.type.name)
                               (interpreted_string_literal) @test.name))))
                 ))))
  )
