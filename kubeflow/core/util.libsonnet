// Some useful routines.
{
  local k = import "k.libsonnet",

  // Convert non-boolean types like string,number to a boolean.
  // This is primarily intended for dealing with parameters that should be booleans.
  toBool:: function(x) {
    result::
      if std.type(x) == "boolean" then
        x
      else if std.type(x) == "string" then
        std.asciiUpper(x) == "TRUE"
      else if std.type(x) == "number" then
        x != 0
      else
        false,
  }.result,

  // Convert a comma-delimited string to an Array
  toArray:: function(str) {
    result::
      if std.type(str) == "string" && str != "null" && std.length(str) > 0 then
        std.split(str, ",")
      else [],
  }.result,

  // Produce a list of manifests. obj must be an array
  list(obj):: k.core.v1.list.new(obj,),
}
