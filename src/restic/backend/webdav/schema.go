package webdav

type Multistatus struct {
	Response []struct {
		HREF string
	}
}

//  <?xml version="1.0" encoding="utf-8"?>
//  <D:multistatus xmlns:D="DAV:">
//  	<D:response xmlns:lp1="DAV:" xmlns:lp2="http://apache.org/dav/props/">
//  		<D:href>/home/mac/keys/</D:href>
//  		<D:propstat>
//  			<D:prop>
//  				<lp1:resourcetype><D:collection/></lp1:resourcetype>
//  				<lp1:creationdate>2016-11-01T18:56:59Z</lp1:creationdate>
//  				<lp1:getlastmodified>Tue, 01 Nov 2016 18:58:02 GMT</lp1:getlastmodified>
//  				<lp1:getetag>"500364-1000-54041e75d72a9"</lp1:getetag>
//  				<D:supportedlock>
//  					<D:lockentry>
//  						<D:lockscope><D:exclusive/></D:lockscope>
//  						<D:locktype><D:write/></D:locktype>
//  					</D:lockentry>
//  					<D:lockentry>
//  						<D:lockscope><D:shared/></D:lockscope>
//  						<D:locktype><D:write/></D:locktype>
//  					</D:lockentry>
//  				</D:supportedlock>
//  				<D:lockdiscovery/>
//  				<D:getcontenttype>httpd/unix-directory</D:getcontenttype>
//  			</D:prop>
//  			<D:status>HTTP/1.1 200 OK</D:status>
//  		</D:propstat>
//  	</D:response>
//  </D:multistatus>
//
