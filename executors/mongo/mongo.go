package mongo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/mitchellh/mapstructure"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.yaml.in/yaml/v3"

	"github.com/ovh/venom"
)

const Name = "mongo"

// testsuiteWorkdirVar is the context variable holding the testsuite's working directory, against
// which every relative path this executor reads (fixtures, the insert file) is resolved.
const testsuiteWorkdirVar = "venom.testsuite.workdir"

func New() venom.Executor {
	return &Executor{}
}

type Executor struct {
	URI        string           `json:"uri,omitempty" yaml:"uri,omitempty"`
	Database   string           `json:"database,omitempty" yaml:"database,omitempty"`
	Collection string           `json:"collection,omitempty" yaml:"collection,omitempty"`
	Actions    []map[string]any `json:"actions,omitempty" yaml:"actions,omitempty"`
}

type Result struct {
	Actions []map[string]any `json:"actions,omitempty" yaml:"actions,omitempty"`
}

func (e Executor) Run(ctx context.Context, step venom.TestStep) (any, error) {
	if err := mapstructure.Decode(step, &e); err != nil {
		return nil, err
	}

	venom.Debug(ctx, "connecting to database: %s\n", redactURI(e.URI))
	mongoClient, err := mongo.Connect(options.Client().ApplyURI(e.URI))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer mongoClient.Disconnect(ctx) //nolint:errcheck // cleanup on exit: Run has already produced its result/error by the time this fires, a disconnect failure has nothing left to affect

	results := make([]map[string]any, len(e.Actions))

	for i, action := range e.Actions {
		result, err := e.runAction(ctx, mongoClient, action)
		if err != nil {
			return nil, err
		}
		results[i] = result
	}

	return Result{Actions: results}, nil
}

// redactURI returns uri with its password replaced by xxxxx, for the log. url.Parse rejects some URIs the driver
// accepts (several IPv6 hosts, a space or a bad escape in the user info) and its error quotes the whole URI, password
// included; it reads a URI without "//" as an opaque string Redacted leaves alone; and it cuts the string at "#" and "?"
// before it looks for the "@" of the user info, so a password or a user name holding one of them parses without an
// error and without a user. In those cases everything between the scheme and the last "@" is cut by hand, and the
// parse error is never used. An "@" in an option of a URI without user info is cut too: hiding too much is accepted.
func redactURI(uri string) string {
	u, err := url.Parse(uri)
	if err == nil && u.Opaque == "" && (u.User != nil || !strings.Contains(uri, "@")) {
		return u.Redacted()
	}
	at := strings.LastIndex(uri, "@")
	if at < 0 {
		return uri
	}
	scheme := ""
	if i := strings.Index(uri, "://"); i >= 0 && i < at {
		scheme = uri[:i+len("://")]
	}
	return scheme + "xxxxx@" + uri[at+1:]
}

// runAction dispatches one action on its type. loadFixtures, dropCollection, createCollection and an unknown type
// have no result: their entry in Result.Actions stays nil.
func (e Executor) runAction(ctx context.Context, mongoClient *mongo.Client, action map[string]any) (map[string]any, error) {
	actionType := fmt.Sprintf("%v", action["type"])
	switch actionType {
	case "loadFixtures":
		return nil, e.loadFixtures(ctx, mongoClient, action)
	case "dropCollection":
		return nil, mongoClient.Database(e.Database).Collection(e.Collection).Drop(ctx)
	case "createCollection":
		return nil, mongoClient.Database(e.Database).CreateCollection(ctx, e.Collection)
	case "count":
		return e.countDocuments(ctx, mongoClient, action)
	case "insert":
		return e.insertDocuments(ctx, mongoClient, action)
	case "find":
		return e.findDocuments(ctx, mongoClient, action)
	case "update":
		return e.updateDocuments(ctx, mongoClient, action)
	case "delete":
		return e.deleteDocuments(ctx, mongoClient, action)
	case "aggregate":
		return e.aggregate(ctx, mongoClient, action)
	}
	return nil, nil
}

// fixture is one decoded fixture file: the collection it fills and the documents it holds.
type fixture struct {
	collection string
	documents  []any
}

// loadFixtures reads every YAML file of the folder, then empties the database, then inserts each file into the
// collection named after it. These fail the step before any collection is listed or dropped: a missing or unreadable
// folder, a fixture file that cannot be opened or decoded as a YAML list, a list that is empty or null, and a list
// with an element that is not a document or that the driver cannot encode (see decodeFixture). Once the database is
// being emptied, the load stops only on a server error (listing, dropping, inserting).
func (e Executor) loadFixtures(ctx context.Context, mongoClient *mongo.Client, action map[string]any) error {
	var loadFixturesAction LoadFixturesAction
	if err := mapstructure.Decode(action, &loadFixturesAction); err != nil {
		return err
	}

	if loadFixturesAction.Folder == "" {
		return fmt.Errorf("folder is required")
	}

	fixtures, err := readFixtures(path.Join(venom.StringVarFromCtx(ctx, testsuiteWorkdirVar), loadFixturesAction.Folder))
	if err != nil {
		return err
	}

	// Only now that every fixture is in memory, drop the existing collections to start clean.
	if err := e.dropAllCollections(ctx, mongoClient); err != nil {
		return err
	}

	for _, fixture := range fixtures {
		if _, err := mongoClient.Database(e.Database).Collection(fixture.collection).InsertMany(ctx, fixture.documents); err != nil {
			return fmt.Errorf("failed to insert documents in collection %s: %w", fixture.collection, err)
		}
	}
	return nil
}

// readFixtures decodes every fixture file of the folder, in directory order.
func readFixtures(folder string) ([]fixture, error) {
	files, err := fixtureFiles(folder)
	if err != nil {
		return nil, err
	}

	fixtures := make([]fixture, 0, len(files))
	for _, file := range files {
		documents, err := decodeFixture(path.Join(folder, file))
		if err != nil {
			return nil, err
		}
		fixtures = append(fixtures, fixture{collection: strings.TrimSuffix(file, path.Ext(file)), documents: documents})
	}
	return fixtures, nil
}

// dropAllCollections drops every collection of the database except the system ones.
func (e Executor) dropAllCollections(ctx context.Context, mongoClient *mongo.Client) error {
	collections, err := mongoClient.Database(e.Database).ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("failed to list collections: %w", err)
	}

	for _, collection := range collections {
		if strings.HasPrefix(collection, "system.") {
			continue
		}

		if err := mongoClient.Database(e.Database).Collection(collection).Drop(ctx); err != nil {
			return fmt.Errorf("failed to drop collection %s: %w", collection, err)
		}
	}
	return nil
}

// fixtureFiles lists the .yaml and .yml files of the folder, in directory order, subfolders skipped.
func fixtureFiles(folder string) ([]string, error) {
	dirEntries, err := os.ReadDir(folder)
	if err != nil {
		return nil, err
	}

	fixtures := make([]string, 0, len(dirEntries))
	for _, file := range dirEntries {
		if file.IsDir() {
			continue
		}

		extension := path.Ext(file.Name())
		if extension != ".yaml" && extension != ".yml" {
			continue
		}
		fixtures = append(fixtures, file.Name())
	}
	return fixtures, nil
}

// decodeFixture decodes one fixture file, which must hold a non-empty YAML list of documents, and closes it.
// A document is a mapping with string keys: yaml.v3 decodes it as map[string]any, the only top-level shape the
// driver's InsertMany encodes. Scalars, lists, nulls and mappings with other keys are refused here, before the
// caller drops anything, instead of by InsertMany after the drop. Elements are numbered from 1 in the error.
func decodeFixture(filePath string) ([]any, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	var documents []any
	if err := yaml.NewDecoder(file).Decode(&documents); err != nil {
		return nil, fmt.Errorf("failed to decode fixture %s: %w", filePath, err)
	}
	if len(documents) == 0 {
		return nil, fmt.Errorf("fixture %s holds no documents", filePath)
	}
	for i, document := range documents {
		if _, ok := document.(map[string]any); !ok {
			return nil, fmt.Errorf("fixture %s: element %d is not a document", filePath, i+1)
		}
		// A document can still hold a value the driver cannot encode (a mapping with a non-string key nested in it):
		// encode it now, so InsertMany cannot refuse it after the drop.
		if _, err := bson.Marshal(document); err != nil {
			return nil, fmt.Errorf("fixture %s: element %d is not a document: %w", filePath, i+1, err)
		}
	}
	return documents, nil
}

func (e Executor) countDocuments(ctx context.Context, mongoClient *mongo.Client, action map[string]any) (map[string]any, error) {
	var countAction CountAction
	if err := mapstructure.Decode(action, &countAction); err != nil {
		return nil, err
	}

	filter := bson.M{}
	if countAction.Filter != "" {
		if err := unmarshalExtJSONM([]byte(countAction.Filter), &filter); err != nil {
			return nil, err
		}
	}

	var countOptions []options.Lister[options.CountOptions]
	if countAction.Options.Limit != nil {
		countOptions = append(countOptions, options.Count().SetLimit(*countAction.Options.Limit))
	}

	count, err := mongoClient.Database(e.Database).Collection(e.Collection).CountDocuments(ctx, filter, countOptions...)
	if err != nil {
		return nil, err
	}

	return map[string]any{"count": count}, nil
}

// insertDocuments inserts the documents of the JSON file first, then the inline ones.
func (e Executor) insertDocuments(ctx context.Context, mongoClient *mongo.Client, action map[string]any) (map[string]any, error) {
	var insertAction InsertAction
	if err := mapstructure.Decode(action, &insertAction); err != nil {
		return nil, err
	}

	documents, err := documentsFromFile(ctx, insertAction.File)
	if err != nil {
		return nil, err
	}

	inlineDocuments, err := documentsFromStrings(insertAction.Documents)
	if err != nil {
		return nil, err
	}
	documents = append(documents, inlineDocuments...)

	insertManyResult, err := mongoClient.Database(e.Database).Collection(e.Collection).InsertMany(ctx, documents)
	if err != nil {
		return nil, err
	}

	// Only InsertedIDs: the driver v2 result also carries Acknowledged, which the v1 result did not.
	return map[string]any{"InsertedIDs": insertManyResult.InsertedIDs}, nil
}

// documentsFromFile reads the JSON documents of the file at filePath, resolved against the testsuite's workdir, and
// decodes each into a bson.M. It returns nil, nil when filePath is empty.
func documentsFromFile(ctx context.Context, filePath string) ([]any, error) {
	if filePath == "" {
		return nil, nil
	}

	file, err := os.Open(path.Join(venom.StringVarFromCtx(ctx, testsuiteWorkdirVar), filePath))
	if err != nil {
		return nil, err
	}

	var documents []any
	decoder := json.NewDecoder(file)
	for {
		var documentJSON any
		if err := decoder.Decode(&documentJSON); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		documentBytes, err := json.Marshal(documentJSON)
		if err != nil {
			return nil, err
		}

		var document bson.M
		if err := unmarshalExtJSONM(documentBytes, &document); err != nil {
			return nil, err
		}
		documents = append(documents, document)
	}
	return documents, nil
}

// documentsFromStrings decodes each extended-JSON string into a bson.M.
func documentsFromStrings(documentsJSON []string) ([]any, error) {
	var documents []any
	for _, documentJSON := range documentsJSON {
		var document bson.M
		if err := unmarshalExtJSONM([]byte(documentJSON), &document); err != nil {
			return nil, err
		}
		documents = append(documents, document)
	}
	return documents, nil
}

func (e Executor) findDocuments(ctx context.Context, mongoClient *mongo.Client, action map[string]any) (map[string]any, error) {
	var findAction FindAction
	if err := mapstructure.Decode(action, &findAction); err != nil {
		return nil, err
	}

	filter := bson.M{}
	if findAction.Filter != "" {
		if err := unmarshalExtJSONM([]byte(findAction.Filter), &filter); err != nil {
			return nil, err
		}
	}

	var findOptions []options.Lister[options.FindOptions]
	if findAction.Options.Limit != nil {
		findOptions = append(findOptions, options.Find().SetLimit(*findAction.Options.Limit))
	}
	if findAction.Options.Skip != nil {
		findOptions = append(findOptions, options.Find().SetSkip(*findAction.Options.Skip))
	}
	if findAction.Options.Sort != "" {
		// bson.D keeps the keys in the written order: the driver refuses a map with several keys for a sort.
		var sort bson.D
		if err := bson.UnmarshalExtJSON([]byte(findAction.Options.Sort), false, &sort); err != nil {
			return nil, err
		}
		findOptions = append(findOptions, options.Find().SetSort(sort))
	}
	if findAction.Options.Projection != "" {
		var projection bson.M
		if err := unmarshalExtJSONM([]byte(findAction.Options.Projection), &projection); err != nil {
			return nil, err
		}
		findOptions = append(findOptions, options.Find().SetProjection(projection))
	}

	cursor, err := mongoClient.Database(e.Database).Collection(e.Collection).Find(ctx, filter, findOptions...)
	if err != nil {
		return nil, err
	}

	result, err := decodeCursor(ctx, cursor)
	if err != nil {
		return nil, err
	}
	return map[string]any{"results": result}, nil
}

func (e Executor) updateDocuments(ctx context.Context, mongoClient *mongo.Client, action map[string]any) (map[string]any, error) {
	var updateAction UpdateAction
	if err := mapstructure.Decode(action, &updateAction); err != nil {
		return nil, err
	}

	filter := bson.M{}
	if updateAction.Filter != "" {
		if err := unmarshalExtJSONM([]byte(updateAction.Filter), &filter); err != nil {
			return nil, err
		}
	}

	update := bson.M{}
	if updateAction.Update != "" {
		if err := unmarshalExtJSONM([]byte(updateAction.Update), &update); err != nil {
			return nil, err
		}
	}

	var updateOptions []options.Lister[options.UpdateManyOptions]
	if updateAction.Options.Upsert != nil {
		updateOptions = append(updateOptions, options.UpdateMany().SetUpsert(*updateAction.Options.Upsert))
	}

	updateResult, err := mongoClient.Database(e.Database).Collection(e.Collection).UpdateMany(ctx, filter, update, updateOptions...)
	if err != nil {
		return nil, err
	}

	// The four fields of the v1 result; the driver v2 result also carries Acknowledged.
	return map[string]any{
		"MatchedCount":  updateResult.MatchedCount,
		"ModifiedCount": updateResult.ModifiedCount,
		"UpsertedCount": updateResult.UpsertedCount,
		"UpsertedID":    updateResult.UpsertedID,
	}, nil
}

func (e Executor) deleteDocuments(ctx context.Context, mongoClient *mongo.Client, action map[string]any) (map[string]any, error) {
	var deleteAction DeleteAction
	if err := mapstructure.Decode(action, &deleteAction); err != nil {
		return nil, err
	}

	filter := bson.M{}
	if deleteAction.Filter != "" {
		if err := unmarshalExtJSONM([]byte(deleteAction.Filter), &filter); err != nil {
			return nil, err
		}
	}

	deleteResult, err := mongoClient.Database(e.Database).Collection(e.Collection).DeleteMany(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Only DeletedCount: the driver v2 result also carries Acknowledged, which the v1 result did not.
	return map[string]any{"DeletedCount": deleteResult.DeletedCount}, nil
}

func (e Executor) aggregate(ctx context.Context, mongoClient *mongo.Client, action map[string]any) (map[string]any, error) {
	var aggregateAction AggregateAction
	if err := mapstructure.Decode(action, &aggregateAction); err != nil {
		return nil, err
	}

	pipeline := bson.A{}
	for _, pipelineItemJSON := range aggregateAction.Pipeline {
		// bson.D keeps the keys in the written order, which a $sort stage on several keys depends on.
		var pipelineItem bson.D
		if err := bson.UnmarshalExtJSON([]byte(pipelineItemJSON), false, &pipelineItem); err != nil {
			return nil, err
		}
		pipeline = append(pipeline, pipelineItem)
	}

	cursor, err := mongoClient.Database(e.Database).Collection(e.Collection).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	result, err := decodeCursor(ctx, cursor)
	if err != nil {
		return nil, err
	}
	return map[string]any{"results": result}, nil
}

// unmarshalExtJSONM decodes extended JSON into v, a bson.M or a pointer to one. The driver v2 decodes a nested
// document into a bson.D by default; the map keeps nested documents as bson.M, as the driver v1 did.
func unmarshalExtJSONM(data []byte, v any) error {
	reader, err := bson.NewExtJSONValueReader(bytes.NewReader(data), false)
	if err != nil {
		return err
	}
	decoder := bson.NewDecoder(reader)
	decoder.DefaultDocumentM()
	return decoder.Decode(v)
}

// decodeCursor reads every document of the cursor, then closes it. Nested documents come back as map[string]any, as
// with the driver v1 (the v2 default is bson.D), arrays as bson.A. No document is a nil list.
func decodeCursor(ctx context.Context, cursor *mongo.Cursor) ([]map[string]any, error) {
	defer cursor.Close(ctx)

	var result []map[string]any
	for cursor.Next(ctx) {
		decoder := bson.NewDecoder(bson.NewDocumentReader(bytes.NewReader(cursor.Current)))
		decoder.DefaultDocumentMap()
		var document map[string]any
		if err := decoder.Decode(&document); err != nil {
			return nil, err
		}
		result = append(result, document)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

type Action struct {
	Type string
}

type CountAction struct {
	Action
	Filter  string
	Options struct {
		Limit *int64
	}
}

type LoadFixturesAction struct {
	Action
	Folder string
}

type InsertAction struct {
	Action
	File      string
	Documents []string
}

type FindAction struct {
	Action
	Filter  string
	Options struct {
		Limit      *int64
		Skip       *int64
		Sort       string
		Projection string
	}
}

type UpdateAction struct {
	Action
	Filter  string
	Update  string
	Options struct {
		Upsert *bool
	}
}

type DeleteAction struct {
	Action
	Filter string
}

type AggregateAction struct {
	Action
	Pipeline []string
}
