package web

// Explicit conversation COPY. Only this allowlisted data DTO crosses instances;
// no session metadata, workspace, tool calls, jobs or provider configuration.
import (
 "crypto/sha256"
 "encoding/hex"
 "sync"
 "gregal/internal/skills"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gregal/internal/companion"
	"gregal/internal/llm"
	"gregal/internal/session"
)

const transferMaxBytes = 8 << 20
const transferMaxMessages = 2000
const transferMaxWinds = 16
const transferMaxSkills = 32
const transferMaxText = 64 << 10

var transferIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,100}$`)
var transferEntityPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
var transferColorPattern = regexp.MustCompile(`^#[a-fA-F0-9]{6}$`)

// Absolute filesystem references in prose are not portable. HTTP URLs remain
// text; this does not fetch them or attempt to identify secrets in user prose.
var transferPrivatePath = regexp.MustCompile("(?i)(^|[\\s\"'`(=])(?:file://[^\\s<>\"'`)]*|[a-z]:[\\\\/][^\\s<>\"'`)]*|/[^\\s<>\"'`)]*)")

func transferClean(text string) string {
	return transferPrivatePath.ReplaceAllString(text, "${1}[local-path]")
}
func transferValidID(id string) bool {
	return len(id) > 0 && len(id) <= 120 && transferEntityPattern.MatchString(id)
}

type transferConsent struct {
	Memory    bool `json:"memory"`
	Knowledge bool `json:"knowledge"`
	Skills    bool `json:"skills"`
}
type transferSkill struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Body        string `json:"body"`
}
type transferWind struct {
	Profile   companion.Wind  `json:"profile"`
	Memory    string          `json:"memory"`
	Knowledge string          `json:"knowledge"`
	Skills    []transferSkill `json:"skills"`
}
type transferGroup struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Members []string `json:"members"`
}
type transferMessage struct {
	Role        string `json:"role"`
	Text        string `json:"text"`
	Ts          int64  `json:"ts"`
	SpeakerID   string `json:"speakerId,omitempty"`
	SpeakerName string `json:"speakerName,omitempty"`
	Kind        string `json:"kind,omitempty"`
}
type transferBundle struct {
	Version int               `json:"version"`
	Kind    string            `json:"kind"`
	ID      string            `json:"id"`
	Winds   []transferWind    `json:"winds"`
	Group   *transferGroup    `json:"group,omitempty"`
	History []transferMessage `json:"history"`
	Consent transferConsent   `json:"consent"`
}

var errTransferLimit = errors.New("conversation exceeds transfer limits; choose a smaller conversation")
var errTransferSource = errors.New("unknown transfer source")

func transferTextOK(text string, max int) bool {
	return len(text) <= max && !strings.ContainsRune(text, '\x00') && transferClean(text) == text
}
func (b transferBundle) validate() error {
	invalid := func() error { return errors.New("invalid transfer bundle") }
	if b.Version != 1 || !transferValidID(b.ID) || (b.Kind != "wind" && b.Kind != "rot") {
		return invalid()
	}
	if len(b.Winds) < 1 || len(b.Winds) > transferMaxWinds || len(b.History) > transferMaxMessages {
		return errTransferLimit
	}
	ids := map[string]bool{}
	for _, w := range b.Winds {
		p := w.Profile
		if !transferValidID(p.ID) || ids[p.ID] || strings.TrimSpace(p.Name) == "" || !transferTextOK(p.Name, 256) || !transferColorPattern.MatchString(p.Color) || !transferTextOK(p.Persona, 16<<10) || !transferTextOK(p.Role, 256) || !transferTextOK(p.Style, 256) || !transferTextOK(p.Squad, 256) || !transferTextOK(p.Specialty, 256) {
			return invalid()
		}
		ids[p.ID] = true
		if len(w.Memory) > companion.MaxWindMemoryBytes || len(w.Knowledge) > companion.MaxWindKnowledgeBytes || len(w.Skills) > transferMaxSkills {
			return errTransferLimit
		}
		if !transferTextOK(w.Memory, companion.MaxWindMemoryBytes) || !transferTextOK(w.Knowledge, companion.MaxWindKnowledgeBytes) || (!b.Consent.Memory && w.Memory != "") || (!b.Consent.Knowledge && w.Knowledge != "") || (!b.Consent.Skills && len(w.Skills) > 0) {
			return invalid()
		}
		names := map[string]bool{}
		for _, sk := range w.Skills {
			if !validWindSkillName(sk.Name) || names[strings.ToLower(sk.Name)] || !transferTextOK(sk.Name, 160) || !transferTextOK(sk.Title, 256) || !transferTextOK(sk.Description, 2048) || strings.ContainsAny(sk.Title+sk.Description, "\r\n") || strings.TrimSpace(sk.Description) == "" || strings.TrimSpace(sk.Body) == "" || !transferTextOK(sk.Body, transferMaxText) {
				return invalid()
			}
			names[strings.ToLower(sk.Name)] = true
		}
	}
	if b.Kind == "wind" {
		if len(b.Winds) != 1 || b.Winds[0].Profile.ID != b.ID || b.Group != nil {
			return invalid()
		}
	} else {
		g := b.Group
		if g == nil || g.ID != b.ID || strings.TrimSpace(g.Name) == "" || !transferTextOK(g.Name, 256) || len(g.Members) != len(b.Winds) {
			return invalid()
		}
		seen := map[string]bool{}
		for _, id := range g.Members {
			if !ids[id] || seen[id] {
				return invalid()
			}
			seen[id] = true
		}
	}
	for _, m := range b.History {
		if (m.Role != "user" && m.Role != "assistant") || !transferTextOK(m.Text, transferMaxText) || m.Ts < 0 || m.Ts > 253402300799 || !transferTextOK(m.SpeakerName, 256) {
			return invalid()
		}
		if (m.Role == "user" && (m.SpeakerID != "" || m.SpeakerName != "" || m.Kind != "")) || (m.SpeakerID != "" && !ids[m.SpeakerID]) || (m.Kind != "" && m.Kind != "debate" && m.Kind != "question" && m.Kind != "result") {
			return invalid()
		}
	}
	raw, _ := json.Marshal(b)
	if len(raw) > transferMaxBytes-(1<<10) {
		return errTransferLimit
	}
	return nil
}

// Stat before using existing readers: never load unbounded files for export.
func transferCheckFile(path string, max int64) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > max {
		return errTransferLimit
	}
	return nil
}
func (s *Server) transferHistory(kind, id string) ([]llm.Message, error) {
	prefix := "company-"
	if kind == "rot" {
		prefix = "rotllana-"
	}
	path := filepath.Join(s.sessDir(), prefix+id+".json")
	if err := transferCheckFile(path, transferMaxBytes); err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return []llm.Message{}, nil
	} else if err != nil {
		return nil, err
	}
	saved, err := session.Load(s.sessDir(), prefix+id)
	return saved.Convo, err
}
func (s *Server) exportTransfer(kind, id string, consent transferConsent) (transferBundle, error) {
	b := transferBundle{Version: 1, Kind: kind, ID: id, Consent: consent, Winds: []transferWind{}, History: []transferMessage{}}
	if !transferValidID(id) || (kind != "wind" && kind != "rot") {
		return b, errors.New("invalid kind or id")
	}
	ids := []string{id}
	if kind == "rot" {
		rot, ok := s.rotllanaByID(id)
		if !ok {
			return b, errTransferSource
		}
		b.Group = &transferGroup{ID: id, Name: transferClean(rot.Name), Members: rot.Members}
		ids = rot.Members
	}
	if len(ids) > transferMaxWinds {
		return b, errTransferLimit
	}
	for _, wid := range ids {
		p, ok := s.winds.ByID(wid)
		if !ok {
			return b, errTransferSource
		}
		p.Name = transferClean(p.Name)
		p.Persona = transferClean(p.Persona)
		p.Role = transferClean(p.Role)
		p.Squad = transferClean(p.Squad)
		p.Specialty = transferClean(p.Specialty)
		p.Style = transferClean(p.Style)
		tw := transferWind{Profile: p, Skills: []transferSkill{}}
		if consent.Memory {
			if err := transferCheckFile(filepath.Join(companion.MemoryDir(s.windStateDir()), wid+".md"), companion.MaxWindMemoryBytes); err != nil {
				return b, err
			}
			tw.Memory = transferClean(companion.LoadWindMemory(s.windStateDir(), wid))
		}
		if consent.Knowledge {
			if err := transferCheckFile(filepath.Join(companion.BrainDir(s.windStateDir()), wid+".md"), companion.MaxWindKnowledgeBytes); err != nil {
				return b, err
			}
			tw.Knowledge = transferClean(s.windBrain(wid))
		}
		if consent.Skills {
			entries, err := os.ReadDir(s.windSkillsDir(wid))
			if err != nil && !os.IsNotExist(err) {
				return b, err
			}
			if len(entries) > transferMaxSkills {
				return b, errTransferLimit
			}
			for _, entry := range entries {
				if err := transferCheckFile(filepath.Join(s.windSkillsDir(wid), entry.Name()), transferMaxText+4096); err != nil {
					return b, err
				}
			}
			for _, sk := range s.windSkills(wid) {
				tw.Skills = append(tw.Skills, transferSkill{Name: sk.Nom, Title: transferClean(sk.Titol), Description: transferClean(sk.Descripcio), Body: transferClean(sk.Cos)})
			}
		}
		b.Winds = append(b.Winds, tw)
	}
	messages, err := s.transferHistory(kind, id)
	if err != nil {
		return b, err
	}
	// Reuse the group converter's legacy speaker resolution, after removing all
	// non-visible/tool messages. No raw Message is ever serialized into a bundle.
	visible := []llm.Message{}
	for _, m := range messages {
		if (m.Role == "user" || m.Role == "assistant") && m.Content != "" {
			visible = append(visible, m)
		}
	}
	if len(visible) > transferMaxMessages {
		return b, errTransferLimit
	}
	rows := s.rotllanaHistoryJSON(visible)
	members := map[string]bool{}
	for _, wid := range ids {
		members[wid] = true
	}
	for i, m := range visible {
		tm := transferMessage{Role: m.Role, Text: transferClean(m.Content), Ts: m.Ts}
		if m.Role == "assistant" {
			tm.Kind = m.Kind
			speaker, _ := rows[i]["speaker_id"].(string)
			if members[speaker] {
				tm.SpeakerID = speaker
				tm.SpeakerName = transferClean(rows[i]["speaker_name"].(string))
			}
		}
		b.History = append(b.History, tm)
	}
	return b, b.validate()
}

func (s *Server) handleWindsTransfer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet:
		consent := transferConsent{}
		for key, dst := range map[string]*bool{"memory": &consent.Memory, "knowledge": &consent.Knowledge, "skills": &consent.Skills} {
			value := r.URL.Query().Get(key)
			if value != "" && value != "true" && value != "false" {
				http.Error(w, "invalid consent flag", 400)
				return
			}
			*dst = value == "true"
		}
		b, err := s.exportTransfer(r.URL.Query().Get("kind"), r.URL.Query().Get("id"), consent)
		if err != nil {
			code := 400
			if errors.Is(err, errTransferLimit) {
				code = 413
			} else if errors.Is(err, errTransferSource) {
				code = 404
			} else if _, ok := err.(*os.PathError); ok {
				code = 503
			}
			http.Error(w, fmt.Sprintf("could not export conversation: %s", transferClean(err.Error())), code)
			return
		}
		writeJSON(w, b)
	case http.MethodPost:
  var req struct {TransferID string `json:"transferId"`;Bundle transferBundle `json:"bundle"`}
  if !decodeTransfer(w,r,&req){return}
  if !transferIDPattern.MatchString(req.TransferID){http.Error(w,"invalid transferId",400);return}
  if err:=req.Bundle.validate();err!=nil{code:=400;if errors.Is(err,errTransferLimit){code=413};http.Error(w,err.Error(),code);return}
  result,err:=s.importTransfer(req.TransferID,req.Bundle)
  if err!=nil{code:=503;if errors.Is(err,errTransferConflict){code=409};http.Error(w,err.Error(),code);return}
  writeJSON(w,result)
 default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "GET or POST required", 405)
	}
}

// transferMu also serializes requests from different tabs sharing the same
// user's on-disk receipt. Catalogue writers already share windLibraryMu.
var transferMu sync.Mutex
var errTransferConflict = errors.New("transferId conflicts with another bundle or destination was removed")
var errTransferStorage = errors.New("transfer incomplete; retry the identical transferId and bundle to reconcile")

type transferCounts struct {
 Winds int `json:"winds"`
 Messages int `json:"messages"`
 MemoryBytes int `json:"memoryBytes"`
 KnowledgeBytes int `json:"knowledgeBytes"`
 Skills int `json:"skills"`
}
type transferResult struct {
 TransferID string `json:"transferId"`
 Kind string `json:"kind"`
 ID string `json:"id"`
 IDMap map[string]string `json:"idMap"`
 Counts transferCounts `json:"counts"`
 VerifyPath string `json:"verifyPath"`
 Replayed bool `json:"replayed"`
}
type transferReceipt struct {
 Digest string `json:"digest"`
 Phase string `json:"phase"`
 Result transferResult `json:"result"`
}

func transferDigest(b transferBundle) string {raw,_:=json.Marshal(b);sum:=sha256.Sum256(raw);return hex.EncodeToString(sum[:])}

// Receipts contain no conversation text. Persist the allocated identities BEFORE
// writing anything else. Staging writes only to these unpublished, fresh IDs.
// The ready phase is durable BEFORE publication, so a crash/retry never rewrites
// a published profile/history or erases conversation progress on the destination.
func (s *Server) importTransfer(id string,b transferBundle)(transferResult,error) {
 transferMu.Lock();defer transferMu.Unlock()
 windLibraryMu.Lock();defer windLibraryMu.Unlock()
 receiptPath:=filepath.Join(s.windStateDir(),"wind-transfers",id+".json")
 var receipt transferReceipt
 raw,err:=os.ReadFile(receiptPath)
 replay:=err==nil
 if err!=nil&&!os.IsNotExist(err){return transferResult{},errTransferStorage}
 if replay {
  if len(raw)>64<<10||json.Unmarshal(raw,&receipt)!=nil {return transferResult{},errTransferStorage}
  if receipt.Digest!=transferDigest(b){return transferResult{},errTransferConflict}
  if receipt.Result.TransferID!=id||receipt.Result.Kind!=b.Kind||!transferValidID(receipt.Result.ID)||len(receipt.Result.IDMap)!=len(b.Winds)||(receipt.Phase!="staging"&&receipt.Phase!="ready"&&receipt.Phase!="complete") {return transferResult{},errTransferStorage}
  for _,w:=range b.Winds{if !transferValidID(receipt.Result.IDMap[w.Profile.ID]){return transferResult{},errTransferStorage}}
 }else{
  receipt=transferReceipt{Digest:transferDigest(b),Phase:"staging",Result:transferResult{TransferID:id,Kind:b.Kind,IDMap:map[string]string{}}}
  for _,w:=range b.Winds {
   fresh:="copy-w-"+windRecordID()
   // Random IDs are independent of names, and even accidental catalogue/file
   // collisions are refused rather than overwritten.
   if _,ok:=s.winds.ByID(fresh);ok{return transferResult{},errTransferStorage}
   for _,p:=range []string{filepath.Join(companion.MemoryDir(s.windStateDir()),fresh+".md"),filepath.Join(companion.BrainDir(s.windStateDir()),fresh+".md"),s.windSkillsDir(fresh),filepath.Join(s.sessDir(),"company-"+fresh+".json")} {if _,err:=os.Lstat(p);!os.IsNotExist(err){return transferResult{},errTransferStorage}}
   receipt.Result.IDMap[w.Profile.ID]=fresh
  }
  receipt.Result.ID=receipt.Result.IDMap[b.ID]
  if b.Kind=="rot" {
   receipt.Result.ID="copy-r-"+windRecordID()
   if _,ok:=s.rotllanaByID(receipt.Result.ID);ok{return transferResult{},errTransferStorage}
   if _,err:=os.Lstat(filepath.Join(s.sessDir(),"rotllana-"+receipt.Result.ID+".json"));!os.IsNotExist(err){return transferResult{},errTransferStorage}
  }
  if b.Kind=="wind"{receipt.Result.VerifyPath="/api/winds/chat?wind="+receipt.Result.ID}else{receipt.Result.VerifyPath="/api/winds/rotllana?rotlla="+receipt.Result.ID}
  if err:=writeWindJSON(receiptPath,receipt);err!=nil{return transferResult{},errTransferStorage}
 }
 result:=receipt.Result
 cloned:=make([]companion.Wind,0,len(b.Winds))
 for _,w:=range b.Winds{p:=w.Profile;p.ID=result.IDMap[p.ID];cloned=append(cloned,p)}
 messages:=make([]llm.Message,0,len(b.History))
 for _,m:=range b.History{messages=append(messages,llm.Message{Role:m.Role,Content:m.Text,Ts:m.Ts,SpeakerID:result.IDMap[m.SpeakerID],SpeakerName:m.SpeakerName,Kind:m.Kind})}
 if receipt.Phase=="staging" {
  // A staging receipt must never refer to already-published resources.
  for _,p:=range cloned{if _,ok:=s.winds.ByID(p.ID);ok{return result,errTransferStorage}}
  for i,w:=range b.Winds {
   target:=cloned[i]
   if err:=companion.ReplaceWindMemory(s.windStateDir(),target.ID,w.Memory);err!=nil{return result,errTransferStorage}
   if err:=companion.ReplaceWindBrain(s.windStateDir(),target.ID,w.Knowledge);err!=nil{return result,errTransferStorage}
   for _,sk:=range w.Skills{if err:=s.saveWindSkill(target.ID,skills.Skill{Nom:sk.Name,Titol:sk.Title,Descripcio:sk.Description,Cos:sk.Body});err!=nil{return result,errTransferStorage}}
  }
  if b.Kind=="wind" {err=saveWindHistory(s,cloned[0],messages)}else{err=saveRotllanaHistory(s,result.ID,messages)}
  if err!=nil{return result,errTransferStorage}
  // Read durable staged data before allowing publication.
  saved,e:=s.transferHistory(b.Kind,result.ID);if e!=nil||len(saved)!=len(messages){return result,errTransferStorage}
  for i,m:=range saved{expected:=messages[i];if m.Role!=expected.Role||m.Content!=expected.Content||m.Ts!=expected.Ts||m.SpeakerID!=expected.SpeakerID||m.SpeakerName!=expected.SpeakerName||m.Kind!=expected.Kind{return result,errTransferStorage}}
  for i,w:=range b.Winds{
   wid:=cloned[i].ID
   if companion.LoadWindMemory(s.windStateDir(),wid)!=w.Memory||s.windBrain(wid)!=w.Knowledge {return result,errTransferStorage}
   savedSkills:=s.windSkills(wid);if len(savedSkills)!=len(w.Skills){return result,errTransferStorage}
   byName:=map[string]skills.Skill{};for _,sk:=range savedSkills{byName[sk.Nom]=sk}
   for _,sk:=range w.Skills {actual,ok:=byName[sk.Name];if !ok||actual.Titol!=sk.Title||actual.Descripcio!=sk.Description||strings.TrimSpace(actual.Cos)!=strings.TrimSpace(sk.Body){return result,errTransferStorage}}
  }
  receipt.Phase="ready"
  if err:=writeWindJSON(receiptPath,receipt);err!=nil{return result,errTransferStorage}
 }
 if receipt.Phase=="ready" {
  for _,p:=range cloned {
   if _,ok:=s.winds.ByID(p.ID);ok{continue} // already published by this receipt; never overwrite
   if err:=s.winds.Restore(p);err!=nil{return result,errTransferStorage}
  }
  if b.Kind=="rot" {
   members:=make([]string,0,len(b.Group.Members));for _,id:=range b.Group.Members{members=append(members,result.IDMap[id])}
   if err:=s.rotllanes.restoreTransferred(Rotllana{ID:result.ID,Name:b.Group.Name,Members:members});err!=nil{return result,errTransferStorage}
  }
  receipt.Phase="complete"
  if err:=writeWindJSON(receiptPath,receipt);err!=nil{return result,errTransferStorage}
 }
 // Return counts from persisted destination data, not optimistic request totals.
 result.Counts=transferCounts{}
 for _,p:=range cloned {
  if _,ok:=s.winds.ByID(p.ID);!ok{return result,errTransferConflict}
  result.Counts.Winds++
  result.Counts.MemoryBytes+=len(companion.LoadWindMemory(s.windStateDir(),p.ID))
  result.Counts.KnowledgeBytes+=len(s.windBrain(p.ID))
  result.Counts.Skills+=len(s.windSkills(p.ID))
 }
 if b.Kind=="rot" {if _,ok:=s.rotllanaByID(result.ID);!ok{return result,errTransferConflict}}
 prefix:="company-";if b.Kind=="rot"{prefix="rotllana-"}
 if _,err:=os.Stat(filepath.Join(s.sessDir(),prefix+result.ID+".json"));err!=nil{return result,errTransferConflict}
 saved,e:=s.transferHistory(b.Kind,result.ID);if e!=nil{return result,errTransferStorage}
 result.Counts.Messages=len(saved);result.Replayed=replay
 return result,nil
}

// Insert-only counterpart to RotllanaStore.Save, which intentionally rejects
// new explicit IDs. It shares the store's mutex and existing atomic JSON writer;
// only transfer-owned identities are inserted, never a namesake or workspace.
func (rs *RotllanaStore) restoreTransferred(rot Rotllana) error {
 rs.mu.Lock();defer rs.mu.Unlock()
 raw,err:=os.ReadFile(rs.path());if err!=nil&&!os.IsNotExist(err){return err}
 var d rotllanesDisc
 if len(raw)>0{if err:=json.Unmarshal(raw,&d);err!=nil{return err}}
 for _,existing:=range d.Rotllanes{if existing.ID==rot.ID{return nil}}
 d.Rotllanes=append(d.Rotllanes,rot)
 return writeWindJSON(rs.path(),d)
}

// Unlike decodeSmall, imports are larger but still bounded, strict, and one JSON
// object only. Trailing payloads must not accidentally change retry identity.
func decodeTransfer(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, transferMaxBytes))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		http.Error(w, "invalid or oversized transfer request", 400)
		return false
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		http.Error(w, "one JSON object required", 400)
		return false
	}
	return true
}
