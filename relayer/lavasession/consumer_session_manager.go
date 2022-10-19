package lavasession

import (
	"context"
	"math/rand"
	"strconv"
	"sync"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/lavanet/lava/utils"
)

type ConsumerSessionManager struct {
	lock         sync.RWMutex
	pairing      map[string]*ConsumerSessionsWithProvider // key == provider adderss
	currentEpoch uint64

	pairingAdressess []string // contains all addressess from the initial pairing.
	// providerBlockList + validAddressess == pairingAdressess (while locked)
	validAddressess       []string // contains all addressess that are currently valid
	providerBlockList     []string // contains all currently blocked providers, reseted upon epoch change.
	addedToPurgeAndReport []string // list of purged providers to report for QoS unavailability.

	// pairingPurge - contains all pairings that are unwanted this epoch, keeps them in memory in order to avoid release.
	// (if a consumer session still uses one of them or we want to report it.)
	pairingPurge map[string]*ConsumerSessionsWithProvider
}

//
func (cs *ConsumerSessionManager) UpdateAllProviders(ctx context.Context, epoch uint64, pairingList []*ConsumerSessionsWithProvider) error {
	// 1. use epoch in order to update a specific epoch.
	// 2. update epoch itself
	// 3. move current pairings to previous pairings.
	// 4. lock and rewrite pairings.
	// take care of the following case: request a deletion of a provider from an old epoch, if the epoch is older return an error or do nothing
	// 5. providerBlockList reset

	cs.lock.Lock()       // start by locking the class lock.
	defer cs.lock.Lock() // we defer here so in case we return an error it will unlock automatically.

	if epoch <= cs.currentEpoch { // sentry shouldnt update an old epoch or current epoch
		return utils.LavaFormatError("trying to update provider list for older epoch", nil, &map[string]string{"epoch": strconv.FormatUint(epoch, 10)})
	}
	// Reset States
	pairingListLength := len(pairingList)
	cs.validAddressess = make([]string, pairingListLength)
	cs.providerBlockList = make([]string, 0)
	cs.pairingAdressess = make([]string, pairingListLength)
	cs.addedToPurgeAndReport = make([]string, 0)

	// Reset the pairingPurge.
	// This happens only after an entire epoch. so its impossible to have session connected to the old purged list
	cs.pairingPurge = make(map[string]*ConsumerSessionsWithProvider)
	for key, value := range cs.pairing {
		cs.pairingPurge[key] = value
	}
	cs.pairing = make(map[string]*ConsumerSessionsWithProvider)
	for idx, provider := range pairingList {
		cs.pairingAdressess[idx] = provider.Acc
		cs.pairing[provider.Acc] = provider
	}
	copy(cs.validAddressess, cs.pairingAdressess) // the starting point is that valid addressess are equal to pairing addressess.
	cs.currentEpoch = epoch

	return nil
}

// Get a valid provider address.
func (cs *ConsumerSessionManager) getValidProviderAddress() (address string, err error) {
	// cs.Lock must be Rlocked here.
	validAddressessLength := len(cs.validAddressess)
	if validAddressessLength <= 0 {
		err = sdkerrors.Wrapf(PairingListEmpty, "cs.validAddressess is empty")
		return
	}
	address = cs.validAddressess[rand.Intn(validAddressessLength)]
	return
}

//
func (cs *ConsumerSessionManager) GetSession(cuNeededForSession uint64) (clinetSession *ConsumerSession, epoch uint64, err error) {
	// 0. lock pairing for Read only - dont forget to release upon failiures

	// 1. get a random provider from pairing map
	// 2. make sure he responds
	// 		-> if not try different endpoint ->
	// 			->  if yes, make sure no more than X(10 currently) paralel sessions only when new session is needed validate this
	// 			-> if all endpoints are dead get another provider
	// 				-> try again
	// 3. after session is picked / created, we lock it and return it

	// design:
	// random select over providers with retry
	// 	   loop over endpoints
	//         loop over sessions (not black listed [with atomic read because its not locked] and can be locked)

	// UsedComputeUnits - updating the provider of the cu that will be used upon getsession success.
	// if session will fail in the future this amount should be deducted

	// check PairingListEmpty error

	cs.lock.RLock()
	defer cs.lock.RUnlock() // will automatically unlock when returns or error is returned
	for numOfPairings := 0; numOfPairings < len(cs.validAddressess); numOfPairings++ {
		providerAddress, err := cs.getValidProviderAddress()
		if err != nil {
			return nil, 0, utils.LavaFormatError("couldnt get a provider address", err, nil)
		}
		consumerSessionWithProvider := cs.pairing[providerAddress]
		connected, endpoint := consumerSessionWithProvider.fetchEndpointConnectionFromClientWrapper()

		// clinetSession, epoch, err = cs.getSessionFromAProvider(providerAddress, cuNeededForSession)
		// if err != nil {

		// }
	}

	return
}

// report a failure with the provider.
func (cs *ConsumerSessionManager) providerBlock(address string, reportProvider bool) error {
	// read currentEpoch atomic if its the same we need to lock and read again.
	// validate errorReceived, some errors will blocklist some will not. if epoch is not older than currentEpoch.
	// checks here for anything changed while waiting for lock (epoch / pairing doesnt excisits anymore etc..)
	// validate the error

	// validate the provider is not already blocked as two sessions can report same provider at the same time
	return nil
}

func (cs *ConsumerSessionManager) SessionFailure(clientSession *ConsumerSession, errorReceived error) error {
	// clientSession must be locked when getting here. verify.

	// client Session should be locked here. so we can just apply the session failure here.
	if clientSession.blocklisted {
		// if client session is already blocklisted return an error.
		return utils.LavaFormatError("trying to report a session failure of a blocklisted client session", nil, &map[string]string{"clientSession.blocklisted": strconv.FormatBool(clientSession.blocklisted)})
	}
	// 1. if we failed we need to update the session UsedComputeUnits. -> lock RelayerClientWrapper to modify it
	// 2. clientSession.blocklisted = true
	// 3. report provider if needed. check cases.
	// unlock clientSession.
	return nil
}

// get a session from a specific provider, pairing must be locked before accessing here.
func (cs *ConsumerSessionManager) getSessionFromAProvider(address string, cuNeeded uint64) (clinetSession *ConsumerSession, epoch uint64, err error) {
	// get session inner function, to get a session from a specific provider.
	// get address from -> pairing map[string]*RelayerClientWrapper ->
	// choose a endpoint for the provider: similar to findPairing.
	// for _, session := range wrap.Sessions {
	// 	if session.Endpoint != endpoint {
	// 		//skip sessions that don't belong to the active connection
	// 		continue
	// 	}
	// 	if session.Lock.TryLock() {
	// 		return session
	// 	}
	// }

	return nil, cs.currentEpoch, nil // TODO_RAN: switch cs.currentEpoch to atomic read
}

func (cs *ConsumerSessionManager) getEndpointFromProvider(address string) (connected bool, endpointPtr *Endpoint) {
	// get the code from sentry FetchEndpointConnectionFromClientWrapper
	return false, nil
}

// get a session from the pool except a specific providers
func (cs *ConsumerSessionManager) GetSessionFromAllExcept(bannedAddresses []string, cuNeeded uint64, bannedAddressessEpoch uint64) (clinetSession *ConsumerSession, epoch uint64, err error) {
	// if bannedAddressessEpoch != current epoch, we just return GetSession. locks...

	// similar to GetSession code. (they should have same inner function)
	return nil, cs.currentEpoch, nil
}

func (cs *ConsumerSessionManager) DoneWithSession(epoch uint64, qosInfo QoSInfo, latestServicedBlock uint64) error {
	// release locks, update CU, relaynum etc..
	// apply LatestRelayCu to CuSum and reset
	// apply QoS
	// apply RelayNum + 1
	// update serviced node LatestBlock (ETH, etc.. ) <- latestServicedBlock
	// unlock clientSession.
	return nil
}

func (cs *ConsumerSessionManager) GetReportedProviders(epoch uint64) (string, error) {
	// Rlock providerBlockList
	// return providerBlockList
	return "", nil
}
