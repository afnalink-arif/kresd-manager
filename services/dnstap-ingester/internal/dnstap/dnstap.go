// Package dnstap contains the protobuf-generated types for dnstap.
// Simplified version — in production, generate from dnstap.proto.
package dnstap

type SocketProtocol int32

const (
	SocketProtocol_UDP          SocketProtocol = 1
	SocketProtocol_TCP          SocketProtocol = 2
	SocketProtocol_DOT          SocketProtocol = 3
	SocketProtocol_DOH          SocketProtocol = 4
	SocketProtocol_DNSCryptUDP  SocketProtocol = 5
	SocketProtocol_DNSCryptTCP  SocketProtocol = 6
)

type MessageType int32

const (
	MessageType_AUTH_QUERY         MessageType = 1
	MessageType_AUTH_RESPONSE      MessageType = 2
	MessageType_RESOLVER_QUERY     MessageType = 3
	MessageType_RESOLVER_RESPONSE  MessageType = 4
	MessageType_CLIENT_QUERY       MessageType = 5
	MessageType_CLIENT_RESPONSE    MessageType = 6
	MessageType_FORWARDER_QUERY    MessageType = 7
	MessageType_FORWARDER_RESPONSE MessageType = 8
)

type Message struct {
	Type             *MessageType    `protobuf:"varint,1,req,name=type,enum=dnstap.Message_Type" json:"type,omitempty"`
	SocketFamily     *int32          `protobuf:"varint,2,opt,name=socket_family" json:"socket_family,omitempty"`
	SocketProtocol   *SocketProtocol `protobuf:"varint,3,opt,name=socket_protocol,enum=dnstap.SocketProtocol" json:"socket_protocol,omitempty"`
	QueryAddress     []byte          `protobuf:"bytes,4,opt,name=query_address" json:"query_address,omitempty"`
	ResponseAddress  []byte          `protobuf:"bytes,5,opt,name=response_address" json:"response_address,omitempty"`
	QueryPort        *uint32         `protobuf:"varint,6,opt,name=query_port" json:"query_port,omitempty"`
	ResponsePort     *uint32         `protobuf:"varint,7,opt,name=response_port" json:"response_port,omitempty"`
	QueryTimeSec     *uint64         `protobuf:"varint,8,opt,name=query_time_sec" json:"query_time_sec,omitempty"`
	QueryTimeNsec    *uint32         `protobuf:"fixed32,9,opt,name=query_time_nsec" json:"query_time_nsec,omitempty"`
	QueryMessage     []byte          `protobuf:"bytes,10,opt,name=query_message" json:"query_message,omitempty"`
	ResponseTimeSec  *uint64         `protobuf:"varint,11,opt,name=response_time_sec" json:"response_time_sec,omitempty"`
	ResponseTimeNsec *uint32         `protobuf:"fixed32,12,opt,name=response_time_nsec" json:"response_time_nsec,omitempty"`
	ResponseMessage  []byte          `protobuf:"bytes,13,opt,name=response_message" json:"response_message,omitempty"`
}

func (m *Message) GetType() MessageType {
	if m != nil && m.Type != nil {
		return *m.Type
	}
	return MessageType_AUTH_QUERY
}

func (m *Message) GetSocketProtocol() SocketProtocol {
	if m != nil && m.SocketProtocol != nil {
		return *m.SocketProtocol
	}
	return SocketProtocol_UDP
}

func (m *Message) GetQueryAddress() []byte     { if m != nil { return m.QueryAddress }; return nil }
func (m *Message) GetResponseAddress() []byte  { if m != nil { return m.ResponseAddress }; return nil }
func (m *Message) GetQueryMessage() []byte     { if m != nil { return m.QueryMessage }; return nil }
func (m *Message) GetResponseMessage() []byte  { if m != nil { return m.ResponseMessage }; return nil }
func (m *Message) GetQueryTimeSec() uint64     { if m != nil && m.QueryTimeSec != nil { return *m.QueryTimeSec }; return 0 }
func (m *Message) GetQueryTimeNsec() uint32    { if m != nil && m.QueryTimeNsec != nil { return *m.QueryTimeNsec }; return 0 }
func (m *Message) GetResponseTimeSec() uint64  { if m != nil && m.ResponseTimeSec != nil { return *m.ResponseTimeSec }; return 0 }
func (m *Message) GetResponseTimeNsec() uint32 { if m != nil && m.ResponseTimeNsec != nil { return *m.ResponseTimeNsec }; return 0 }

type Dnstap struct {
	Identity []byte   `protobuf:"bytes,1,opt,name=identity" json:"identity,omitempty"`
	Version  []byte   `protobuf:"bytes,2,opt,name=version" json:"version,omitempty"`
	Type     *int32   `protobuf:"varint,3,req,name=type" json:"type,omitempty"`
	Message  *Message `protobuf:"bytes,14,opt,name=message" json:"message,omitempty"`
}

func (d *Dnstap) GetMessage() *Message {
	if d != nil {
		return d.Message
	}
	return nil
}

func (d *Dnstap) ProtoReflect() {}
func (d *Dnstap) Reset()        { *d = Dnstap{} }
func (d *Dnstap) String() string { return "" }
