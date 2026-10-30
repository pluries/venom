package yamljson

// The tests call the package through these four names. They are the seam that lets the same corpus
// run against another implementation: point them at github.com/rockbears/yaml (Unmarshal, Marshal,
// JSONToYAML, YAMLToJSON) to compare behaviour, as was done when the port was written.
var (
	unmarshal  = Unmarshal
	marshal    = Marshal
	jsonToYAML = JSONToYAML
	yamlToJSON = YAMLToJSON
)
