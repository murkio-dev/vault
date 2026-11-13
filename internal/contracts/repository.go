package contracts

import (
	"context"
	"time"
	"uuid"

	"github.com/murkio-dev/vault/internal/entities"
)

// Os contratos de repository são pequenos e concretos: um por grupo de métodos.
// O que muda o resultado (aplicação, dono, filtro, hash) é parâmetro explícito
// e obrigatório: uuid zero é recusado pelo repository, nunca lido como "sem
// filtro". Registro não encontrado devolve sql.ErrNoRows.

// Aplicações

type IApplicationCreator interface {
	Create(ctx context.Context, application entities.Application) (entities.Application, error)
}

// IApplicationChecker diz se já há uma aplicação ativa com o nome.
type IApplicationChecker interface {
	Exist(ctx context.Context, application entities.Application) (bool, error)
}

type IApplicationFinder interface {
	FindById(ctx context.Context, id uuid.UUID) (entities.Application, error)
}

// IApplicationLister lista todas as aplicações ativas (só a plataforma).
type IApplicationLister interface {
	List(ctx context.Context, limit, offset int) ([]entities.Application, int, error)
}

// IApplicationUserLister lista só as aplicações de que a conta é membro.
type IApplicationUserLister interface {
	ListByUser(ctx context.Context, user uuid.UUID, limit, offset int) ([]entities.Application, int, error)
}

// IApplicationUpdater é a substituição completa (PUT).
type IApplicationUpdater interface {
	Update(ctx context.Context, application entities.Application) (entities.Application, error)
}

// IApplicationUpgrader é a atualização parcial (PATCH).
type IApplicationUpgrader interface {
	Upgrade(ctx context.Context, application entities.Application) (entities.Application, error)
}

type IApplicationDeleter interface {
	Delete(ctx context.Context, id uuid.UUID) error
}

// Membros

type IMemberCreator interface {
	Create(ctx context.Context, member entities.Member) (entities.Member, error)
}

type IMemberFinder interface {
	FindById(ctx context.Context, application, id uuid.UUID) (entities.Member, error)
}

// IMemberUserFinder acha o vínculo de uma conta com a aplicação.
type IMemberUserFinder interface {
	FindByUser(ctx context.Context, application, user uuid.UUID) (entities.Member, error)
}

type IMemberLister interface {
	List(ctx context.Context, application uuid.UUID, limit, offset int) ([]entities.Member, int, error)
}

// IMemberAdminCounter conta os admins ativos da aplicação.
type IMemberAdminCounter interface {
	CountActiveAdmins(ctx context.Context, application uuid.UUID) (int, error)
}

type IMemberUpgrader interface {
	Upgrade(ctx context.Context, member entities.Member) (entities.Member, error)
}

type IMemberDeleter interface {
	Delete(ctx context.Context, application, id uuid.UUID) error
}

// Convites

type IInvitationCreator interface {
	Create(ctx context.Context, invitation entities.Invitation) (entities.Invitation, error)
}

type IInvitationLister interface {
	List(ctx context.Context, application uuid.UUID, limit, offset int) ([]entities.Invitation, int, error)
}

// IInvitationTokenFinder acha o convite pelo hash do token (a credencial de
// quem aceita ainda não tem aplicação).
type IInvitationTokenFinder interface {
	FindByTokenHash(ctx context.Context, tokenHash string) (entities.Invitation, error)
}

type IInvitationAcceptor interface {
	Accept(ctx context.Context, invitation entities.Invitation) (entities.Invitation, error)
}

type IInvitationRevoker interface {
	Revoke(ctx context.Context, application, id uuid.UUID) error
}

// Segredos

type ISecretCreator interface {
	Create(ctx context.Context, write entities.SecretWrite) (entities.Secret, error)
}

// ISecretFinder acha o segredo na aplicação, de qualquer dono (referências e
// leitura do valor). Quem lê em nome de um membro usa ISecretOwnerFinder.
type ISecretFinder interface {
	FindById(ctx context.Context, application, id uuid.UUID) (entities.Secret, error)
}

// ISecretOwnerFinder só acha o segredo da tabela do dono.
type ISecretOwnerFinder interface {
	FindByOwner(ctx context.Context, application, owner, id uuid.UUID) (entities.Secret, error)
}

// ISecretLister lista a tabela de um membro.
type ISecretLister interface {
	List(ctx context.Context, application, owner uuid.UUID, limit, offset int) ([]entities.Secret, int, error)
}

// ISecretCounter conta os segredos da tabela de um membro.
type ISecretCounter interface {
	Count(ctx context.Context, application, owner uuid.UUID) (int, error)
}

// ISecretReferenceCounter conta os segredos que apontam para o segredo.
type ISecretReferenceCounter interface {
	CountReferences(ctx context.Context, application, secret uuid.UUID) (int, error)
}

// ISecretVersionAdder grava uma NOVA versão do valor: a seguinte à
// Secret.Version. Duas escritas na mesma versão: a segunda recebe sql.ErrNoRows.
type ISecretVersionAdder interface {
	AddVersion(ctx context.Context, write entities.SecretWrite) (entities.Secret, error)
}

// ISecretRenamer renomeia o segredo; o valor e as versões não mudam.
type ISecretRenamer interface {
	Rename(ctx context.Context, write entities.SecretWrite) (entities.Secret, error)
}

type ISecretDeleter interface {
	Delete(ctx context.Context, application, id uuid.UUID) error
}

// Versões

type IVersionLister interface {
	List(ctx context.Context, application, secret uuid.UUID, limit, offset int) ([]entities.SecretVersion, int, error)
}

// IVersionCurrentFinder acha a versão atual do segredo.
type IVersionCurrentFinder interface {
	FindCurrent(ctx context.Context, application, secret uuid.UUID) (entities.SecretVersion, error)
}

// IVersionNumberFinder acha uma versão pelo número.
type IVersionNumberFinder interface {
	FindByNumber(ctx context.Context, application, secret uuid.UUID, number int) (entities.SecretVersion, error)
}

// IVersionResealer troca o ciphertext de uma versão (a recifragem com a chave
// ativa); o resto da versão não muda.
type IVersionResealer interface {
	Reseal(ctx context.Context, application, secret uuid.UUID, version entities.SecretVersion) (entities.SecretVersion, error)
}

// Tokens do SDK

type ITokenCreator interface {
	Create(ctx context.Context, token entities.AccessToken) (entities.AccessToken, error)
}

type ITokenFinder interface {
	FindById(ctx context.Context, application, id uuid.UUID) (entities.AccessToken, error)
}

type ITokenLister interface {
	List(ctx context.Context, application, member uuid.UUID, limit, offset int) ([]entities.AccessToken, int, error)
}

// ITokenHashFinder é a autenticação do SDK: acha o token válido agora (não
// revogado, não vencido, de membro ativo) pelo hash.
type ITokenHashFinder interface {
	FindActiveByHash(ctx context.Context, tokenHash string) (entities.AccessToken, error)
}

// ITokenUsageToucher marca o último uso (AccessToken.LastUsedAt).
type ITokenUsageToucher interface {
	TouchLastUsed(ctx context.Context, token entities.AccessToken) (entities.AccessToken, error)
}

// ITokenRevoker revoga o token; é idempotente.
type ITokenRevoker interface {
	Revoke(ctx context.Context, application, id uuid.UUID) error
}

// Trilha e outbox

type IAuditLogCreator interface {
	Create(ctx context.Context, log entities.AuditLog) (entities.AuditLog, error)
}

// IAuditLogLister lê a trilha do mais novo para o mais antigo, com cursor em
// filter.Cursor. filter.ApplicationID é obrigatório.
type IAuditLogLister interface {
	List(ctx context.Context, filter entities.AuditFilter, limit int) ([]entities.AuditLog, error)
}

// IAuditLogPurger apaga da trilha o que passou da retenção. PurgeExpired apaga
// até limit linhas de até agora - AuditRetention - margem (now vem de quem
// chama, para o teste e o job controlarem o relógio) e devolve quantas apagou;
// menos que limit diz que não sobrou nada vencido. É a única escrita que a
// trilha aceita além do Create, e o trigger do banco recusa qualquer linha
// dentro da retenção, então um engano aqui nunca apaga auditoria no prazo.
type IAuditLogPurger interface {
	PurgeExpired(ctx context.Context, now time.Time, limit int) (deleted int64, err error)
}

type IOutboxCreator interface {
	Create(ctx context.Context, event entities.OutboxEvent) (entities.OutboxEvent, error)
}

// IAuditedTx roda fn numa transação e grava entry na trilha na MESMA
// transação: se fn falhar, a auditoria não é gravada; se a auditoria falhar, a
// mudança é desfeita. É a forma de todo use case alterar algo, e o que torna a
// auditoria fail-closed.
type IAuditedTx interface {
	Run(ctx context.Context, entry entities.AuditLog, fn func(ctx context.Context) error) error
}

// ISecretValues cifra e decifra os valores. A chave nunca sai do processo.
// Seal cifra o valor de uma versão (versionID entra no AAD); Value devolve o
// valor de uma versão do segredo (0 = a atual), resolvendo a referência e
// recifrando com a chave ativa o que foi aberto com uma antiga.
type ISecretValues interface {
	Seal(versionID uuid.UUID, value string) ([]byte, error)
	Value(ctx context.Context, secret entities.Secret, version int) (value string, number int, err error)
}

// IAccessGuard é o use case de autorização (member/useCases/authorize.go),
// injetado pelo initializr raiz nos demais services. Não é um contrato novo: é
// o IUseCase com os tipos dele.
type IAccessGuard = IUseCase[entities.AuthorizeInput, entities.AuthorizeOutput]

// Shared reúne o que mais de um service recebe do initializr raiz. Cada service
// monta o próprio repository com o db; aqui só entra o que atravessa services.
// Só o initializr raiz conhece as implementações.
type Shared struct {
	Guard            IAccessGuard
	Tx               IAuditedTx
	Values           ISecretValues
	AuditCreator     IAuditLogCreator
	Outbox           IOutboxCreator
	MemberFinder     IMemberFinder
	MemberUserFinder IMemberUserFinder
	MemberCreator    IMemberCreator
	SecretLister     ISecretLister
}
