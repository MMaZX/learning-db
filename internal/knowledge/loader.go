// Package knowledge carga la documentación funcional (Markdown) que describe
// cómo funciona el sistema: qué representa cada entidad de negocio, qué
// tablas la componen y dónde está documentada en detalle.
//
// Todo este conocimiento vive en archivos de texto plano bajo
// MCP_KNOWLEDGE_PATH. Enseñarle algo nuevo al MCP es editar o añadir un
// archivo .md y llamar a reload — nunca recompilar el binario.
package knowledge

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Entity es un concepto de negocio (Venta, Cliente, Kardex...) declarado en
// index.md, con las tablas que lo componen y el documento donde se explica
// en detalle.
type Entity struct {
	Name        string   `json:"name"`
	Tables      []string `json:"tables,omitempty"`
	Document    string   `json:"document,omitempty"`
	Description string   `json:"description,omitempty"`
	// Relations son relaciones semánticas declaradas explícitamente en el
	// índice (ej. "usa: Pedido", "genera: Pago"), no inferidas.
	Relations map[string]string `json:"relations,omitempty"`
}

// Document es un archivo .md de conocimiento funcional ya cargado.
type Document struct {
	Path    string `json:"path"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

type Store struct {
	basePath string

	mu        sync.RWMutex
	entities  map[string]Entity // key: nombre en minúsculas
	documents map[string]Document
}

func NewStore(basePath string) *Store {
	return &Store{basePath: basePath}
}

// Reload relee index.md y todos los .md referenciados/existentes bajo
// business/. Es idempotente y segura de llamar en caliente.
func (s *Store) Reload() error {
	entities, err := parseIndex(filepath.Join(s.basePath, "index.md"))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("leyendo knowledge/index.md: %w", err)
	}

	documents, err := loadDocuments(filepath.Join(s.basePath, "business"))
	if err != nil {
		return fmt.Errorf("leyendo knowledge/business: %w", err)
	}

	s.mu.Lock()
	s.entities = entities
	s.documents = documents
	s.mu.Unlock()
	return nil
}

func (s *Store) ensureLoaded() {
	s.mu.RLock()
	loaded := s.entities != nil
	s.mu.RUnlock()
	if !loaded {
		_ = s.Reload()
	}
}

// Entity devuelve la entidad de negocio por nombre (case-insensitive).
func (s *Store) Entity(name string) (Entity, bool) {
	s.ensureLoaded()
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entities[strings.ToLower(name)]
	return e, ok
}

func (s *Store) AllEntities() []Entity {
	s.ensureLoaded()
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Entity, 0, len(s.entities))
	for _, e := range s.entities {
		out = append(out, e)
	}
	return out
}

// Document busca un documento de negocio por tema (nombre de archivo o
// título, substring case-insensitive).
func (s *Store) Document(topic string) (Document, bool) {
	s.ensureLoaded()
	s.mu.RLock()
	defer s.mu.RUnlock()

	topicLower := strings.ToLower(topic)
	// match exacto por nombre de archivo primero
	if d, ok := s.documents[topicLower]; ok {
		return d, true
	}
	if d, ok := s.documents[topicLower+".md"]; ok {
		return d, true
	}
	for key, d := range s.documents {
		if strings.Contains(key, topicLower) || strings.Contains(strings.ToLower(d.Title), topicLower) ||
			strings.Contains(strings.ToLower(d.Content), topicLower) {
			return d, true
		}
	}
	return Document{}, false
}

func (s *Store) AllDocuments() []Document {
	s.ensureLoaded()
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Document, 0, len(s.documents))
	for _, d := range s.documents {
		out = append(out, d)
	}
	return out
}

// Search hace una búsqueda simple por substring sobre entidades y
// documentos. No es semántica: es literal, predecible y auditable.
func (s *Store) Search(query string) (entities []Entity, documents []Document) {
	s.ensureLoaded()
	q := strings.ToLower(query)
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, e := range s.entities {
		if strings.Contains(strings.ToLower(e.Name), q) ||
			strings.Contains(strings.ToLower(e.Description), q) ||
			containsAny(e.Tables, q) {
			entities = append(entities, e)
		}
	}
	for _, d := range s.documents {
		if strings.Contains(strings.ToLower(d.Title), q) || strings.Contains(strings.ToLower(d.Content), q) {
			documents = append(documents, d)
		}
	}
	return entities, documents
}

func containsAny(list []string, q string) bool {
	for _, item := range list {
		if strings.Contains(strings.ToLower(item), q) {
			return true
		}
	}
	return false
}

// parseIndex interpreta knowledge/index.md con una convención simple:
//
//	## Entidades
//	### <Nombre>
//	- tablas: t1, t2, t3
//	- documento: archivo.md
//	- descripcion: texto libre
//	- relacion:usa: Pedido
//
// Cualquier línea "- clave: valor" dentro del bloque de una entidad se
// interpreta como atributo; "relacion:<verbo>: <Entidad>" se interpreta como
// relación semántica declarada explícitamente por un humano.
func parseIndex(path string) (map[string]Entity, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	entities := make(map[string]Entity)
	var current *Entity

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), " \t")
		trimmed := strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(trimmed, "### "):
			if current != nil {
				entities[strings.ToLower(current.Name)] = *current
			}
			name := strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))
			current = &Entity{Name: name, Relations: map[string]string{}}

		case strings.HasPrefix(trimmed, "- ") && current != nil:
			kv := strings.TrimPrefix(trimmed, "- ")

			if strings.HasPrefix(kv, "relacion:") {
				rest := strings.TrimPrefix(kv, "relacion:")
				verb, value, ok := strings.Cut(rest, ":")
				if ok {
					current.Relations[strings.TrimSpace(verb)] = strings.TrimSpace(value)
				}
				continue
			}

			key, value, ok := strings.Cut(kv, ":")
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)

			switch {
			case key == "tablas":
				for _, t := range strings.Split(value, ",") {
					t = strings.TrimSpace(t)
					if t != "" {
						current.Tables = append(current.Tables, t)
					}
				}
			case key == "documento":
				current.Document = value
			case key == "descripcion":
				current.Description = value
			}
		}
	}
	if current != nil {
		entities[strings.ToLower(current.Name)] = *current
	}
	return entities, scanner.Err()
}

func loadDocuments(dir string) (map[string]Document, error) {
	docs := make(map[string]Document)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return docs, nil
	}
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		title := entry.Name()
		if t := firstHeading(string(content)); t != "" {
			title = t
		}
		key := strings.ToLower(entry.Name())
		docs[key] = Document{Path: path, Title: title, Content: string(content)}
	}
	return docs, nil
}

func firstHeading(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}
