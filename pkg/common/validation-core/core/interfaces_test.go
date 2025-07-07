package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockValidator implements the Validator interface for testing
type MockValidator struct {
	mock.Mock
}

func (m *MockValidator) Validate(ctx context.Context, data interface{}, options *ValidationOptions) *NonGenericResult {
	args := m.Called(ctx, data, options)
	return args.Get(0).(*NonGenericResult)
}

func (m *MockValidator) GetName() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockValidator) GetVersion() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockValidator) GetSupportedTypes() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

// MockGenericValidator implements the GenericValidator interface for testing
type MockGenericValidator[T any] struct {
	mock.Mock
}

func (m *MockGenericValidator[T]) Validate(ctx context.Context, data T, options *ValidationOptions) *Result[T] {
	args := m.Called(ctx, data, options)
	return args.Get(0).(*Result[T])
}

func (m *MockGenericValidator[T]) GetName() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockGenericValidator[T]) GetVersion() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockGenericValidator[T]) GetSupportedTypes() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

// MockValidatorRegistry implements the ValidatorRegistry interface for testing
type MockValidatorRegistry struct {
	mock.Mock
}

func (m *MockValidatorRegistry) Register(name string, validator Validator) error {
	args := m.Called(name, validator)
	return args.Error(0)
}

func (m *MockValidatorRegistry) Unregister(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *MockValidatorRegistry) Get(name string) (Validator, bool) {
	args := m.Called(name)
	return args.Get(0).(Validator), args.Bool(1)
}

func (m *MockValidatorRegistry) List() map[string]Validator {
	args := m.Called()
	return args.Get(0).(map[string]Validator)
}

func (m *MockValidatorRegistry) GetByType(dataType string) []Validator {
	args := m.Called(dataType)
	return args.Get(0).([]Validator)
}

func (m *MockValidatorRegistry) Clear() {
	m.Called()
}

// MockTypedValidatorRegistry implements the TypedValidatorRegistry interface for testing
type MockTypedValidatorRegistry[T any] struct {
	mock.Mock
}

func (m *MockTypedValidatorRegistry[T]) Register(name string, validator GenericValidator[T]) error {
	args := m.Called(name, validator)
	return args.Error(0)
}

func (m *MockTypedValidatorRegistry[T]) Unregister(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *MockTypedValidatorRegistry[T]) Get(name string) (GenericValidator[T], bool) {
	args := m.Called(name)
	return args.Get(0).(GenericValidator[T]), args.Bool(1)
}

func (m *MockTypedValidatorRegistry[T]) List() map[string]GenericValidator[T] {
	args := m.Called()
	return args.Get(0).(map[string]GenericValidator[T])
}

func (m *MockTypedValidatorRegistry[T]) GetByType(dataType string) []GenericValidator[T] {
	args := m.Called(dataType)
	return args.Get(0).([]GenericValidator[T])
}

func (m *MockTypedValidatorRegistry[T]) Clear() {
	m.Called()
}

// MockValidatorChain implements the ValidatorChain interface for testing
type MockValidatorChain struct {
	MockValidator
}

func (m *MockValidatorChain) AddValidator(validator Validator) ValidatorChain {
	args := m.Called(validator)
	return args.Get(0).(ValidatorChain)
}

func (m *MockValidatorChain) RemoveValidator(name string) bool {
	args := m.Called(name)
	return args.Bool(0)
}

func (m *MockValidatorChain) Clear() {
	m.Called()
}

func (m *MockValidatorChain) GetValidators() []Validator {
	args := m.Called()
	return args.Get(0).([]Validator)
}

func TestValidator_Interface(t *testing.T) {
	mockValidator := new(MockValidator)

	// Setup expectations
	mockValidator.On("GetName").Return("test-validator")
	mockValidator.On("GetVersion").Return("1.0.0")
	mockValidator.On("GetSupportedTypes").Return([]string{"test-type"})

	ctx := context.Background()
	options := &ValidationOptions{}
	data := "test data"
	expectedResult := &NonGenericResult{Valid: true}
	mockValidator.On("Validate", ctx, data, options).Return(expectedResult)

	// Test interface methods
	assert.Equal(t, "test-validator", mockValidator.GetName())
	assert.Equal(t, "1.0.0", mockValidator.GetVersion())
	assert.Equal(t, []string{"test-type"}, mockValidator.GetSupportedTypes())

	result := mockValidator.Validate(ctx, data, options)
	assert.Equal(t, expectedResult, result)

	// Verify all expectations were met
	mockValidator.AssertExpectations(t)
}

func TestGenericValidator_Interface(t *testing.T) {
	mockValidator := new(MockGenericValidator[string])

	// Setup expectations
	mockValidator.On("GetName").Return("generic-test-validator")
	mockValidator.On("GetVersion").Return("2.0.0")
	mockValidator.On("GetSupportedTypes").Return([]string{"string"})

	ctx := context.Background()
	options := &ValidationOptions{}
	data := "test string data"
	expectedResult := &Result[string]{Valid: true, Data: data}
	mockValidator.On("Validate", ctx, data, options).Return(expectedResult)

	// Test interface methods
	assert.Equal(t, "generic-test-validator", mockValidator.GetName())
	assert.Equal(t, "2.0.0", mockValidator.GetVersion())
	assert.Equal(t, []string{"string"}, mockValidator.GetSupportedTypes())

	result := mockValidator.Validate(ctx, data, options)
	assert.Equal(t, expectedResult, result)
	assert.Equal(t, data, result.Data)

	// Verify all expectations were met
	mockValidator.AssertExpectations(t)
}

func TestValidatorRegistry_Interface(t *testing.T) {
	mockRegistry := new(MockValidatorRegistry)
	mockValidator := new(MockValidator)

	validatorMap := map[string]Validator{"test": mockValidator}
	validatorList := []Validator{mockValidator}

	// Setup expectations
	mockRegistry.On("Register", "test-validator", mockValidator).Return(nil)
	mockRegistry.On("Unregister", "test-validator").Return(nil)
	mockRegistry.On("Get", "test-validator").Return(mockValidator, true)
	mockRegistry.On("List").Return(validatorMap)
	mockRegistry.On("GetByType", "test-type").Return(validatorList)
	mockRegistry.On("Clear").Return()

	// Test interface methods
	err := mockRegistry.Register("test-validator", mockValidator)
	assert.NoError(t, err)

	err = mockRegistry.Unregister("test-validator")
	assert.NoError(t, err)

	validator, found := mockRegistry.Get("test-validator")
	assert.True(t, found)
	assert.Equal(t, mockValidator, validator)

	list := mockRegistry.List()
	assert.Equal(t, validatorMap, list)

	byType := mockRegistry.GetByType("test-type")
	assert.Equal(t, validatorList, byType)

	mockRegistry.Clear()

	// Verify all expectations were met
	mockRegistry.AssertExpectations(t)
}

func TestTypedValidatorRegistry_Interface(t *testing.T) {
	mockRegistry := new(MockTypedValidatorRegistry[string])
	mockValidator := new(MockGenericValidator[string])

	validatorMap := map[string]GenericValidator[string]{"test": mockValidator}
	validatorList := []GenericValidator[string]{mockValidator}

	// Setup expectations
	mockRegistry.On("Register", "typed-test-validator", mockValidator).Return(nil)
	mockRegistry.On("Unregister", "typed-test-validator").Return(nil)
	mockRegistry.On("Get", "typed-test-validator").Return(mockValidator, true)
	mockRegistry.On("List").Return(validatorMap)
	mockRegistry.On("GetByType", "string").Return(validatorList)
	mockRegistry.On("Clear").Return()

	// Test interface methods
	err := mockRegistry.Register("typed-test-validator", mockValidator)
	assert.NoError(t, err)

	err = mockRegistry.Unregister("typed-test-validator")
	assert.NoError(t, err)

	validator, found := mockRegistry.Get("typed-test-validator")
	assert.True(t, found)
	assert.Equal(t, mockValidator, validator)

	list := mockRegistry.List()
	assert.Equal(t, validatorMap, list)

	byType := mockRegistry.GetByType("string")
	assert.Equal(t, validatorList, byType)

	mockRegistry.Clear()

	// Verify all expectations were met
	mockRegistry.AssertExpectations(t)
}

func TestValidatorChain_Interface(t *testing.T) {
	mockChain := new(MockValidatorChain)
	mockValidator := new(MockValidator)

	// Setup expectations for ValidatorChain specific methods
	mockChain.On("AddValidator", mockValidator).Return(mockChain)
	mockChain.On("RemoveValidator", "test-validator").Return(true)

	// Setup expectations for inherited Validator methods
	mockChain.On("GetName").Return("chain-validator")
	mockChain.On("GetVersion").Return("1.0.0")
	mockChain.On("GetSupportedTypes").Return([]string{"chain-type"})

	ctx := context.Background()
	options := &ValidationOptions{}
	data := "chain test data"
	expectedResult := &NonGenericResult{Valid: true}
	mockChain.On("Validate", ctx, data, options).Return(expectedResult)

	// Test ValidatorChain specific methods
	returnedChain := mockChain.AddValidator(mockValidator)
	assert.Equal(t, mockChain, returnedChain)

	removed := mockChain.RemoveValidator("test-validator")
	assert.True(t, removed)

	// Test inherited Validator methods
	assert.Equal(t, "chain-validator", mockChain.GetName())
	assert.Equal(t, "1.0.0", mockChain.GetVersion())
	assert.Equal(t, []string{"chain-type"}, mockChain.GetSupportedTypes())

	result := mockChain.Validate(ctx, data, options)
	assert.Equal(t, expectedResult, result)

	// Verify all expectations were met
	mockChain.AssertExpectations(t)
}

func TestDomainSpecificValidatorTypeAliases(t *testing.T) {
	// Test that domain-specific validator types are correctly aliased

	// Test BuildValidator
	var buildValidator BuildValidator
	var genericBuildValidator GenericValidator[BuildValidationData]
	assert.IsType(t, genericBuildValidator, buildValidator)

	// Test DeployValidator
	var deployValidator DeployValidator
	var genericDeployValidator GenericValidator[DeployValidationData]
	assert.IsType(t, genericDeployValidator, deployValidator)

	// Test SecurityValidator
	var securityValidator SecurityValidator
	var genericSecurityValidator GenericValidator[SecurityValidationData]
	assert.IsType(t, genericSecurityValidator, securityValidator)

	// Test SessionValidator
	var sessionValidator SessionValidator
	var genericSessionValidator GenericValidator[SessionValidationData]
	assert.IsType(t, genericSessionValidator, sessionValidator)

	// Test RuntimeValidator
	var runtimeValidator RuntimeValidator
	var genericRuntimeValidator GenericValidator[RuntimeValidationData]
	assert.IsType(t, genericRuntimeValidator, runtimeValidator)
}

func TestValidatorInterface_Compatibility(t *testing.T) {
	// Test that legacy Validator interface can be used with ValidatorChain
	mockValidator := new(MockValidator)
	mockChain := new(MockValidatorChain)

	// Validator should be assignable to ValidatorChain (through embedding)
	var validator Validator = mockValidator
	var chain Validator = mockChain

	assert.NotNil(t, validator)
	assert.NotNil(t, chain)

	// ValidatorChain should extend Validator
	var chainAsValidator Validator = mockChain
	assert.NotNil(t, chainAsValidator)
}

func TestGenericValidatorTypeConstraints(t *testing.T) {
	// Test that GenericValidator can handle different types

	// String validator
	var stringValidator GenericValidator[string]
	stringMock := new(MockGenericValidator[string])
	stringValidator = stringMock
	assert.NotNil(t, stringValidator)

	// Int validator
	var intValidator GenericValidator[int]
	intMock := new(MockGenericValidator[int])
	intValidator = intMock
	assert.NotNil(t, intValidator)

	// Struct validator
	type TestStruct struct {
		Field string
	}
	var structValidator GenericValidator[TestStruct]
	structMock := new(MockGenericValidator[TestStruct])
	structValidator = structMock
	assert.NotNil(t, structValidator)

	// Map validator
	var mapValidator GenericValidator[map[string]interface{}]
	mapMock := new(MockGenericValidator[map[string]interface{}])
	mapValidator = mapMock
	assert.NotNil(t, mapValidator)
}
